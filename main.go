package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"time"

	"gocv.io/x/gocv"
	"poc-camera/config"
	"poc-camera/internal/shoplifting"
)

func init() {
	// Lock thread for macOS OpenCV compatibility
	runtime.LockOSThread()
}

// Variável global para configuração
var appConfig *config.Config

// DetectionResult representa uma detecção de objeto
type DetectionResult struct {
	ClassID    int
	Confidence float32
	Box        image.Rectangle
	Label      string
}

// YOLODetector encapsula a lógica de detecção
type YOLODetector struct {
	net        gocv.Net
	classNames []string
	config     *config.Config
}

// YOLODetectorAdapter adapta YOLODetector para shoplifting.ObjectDetector
type YOLODetectorAdapter struct {
	detector *YOLODetector
}

// NewYOLODetectorAdapter cria novo adapter
func NewYOLODetectorAdapter(detector *YOLODetector) *YOLODetectorAdapter {
	return &YOLODetectorAdapter{detector: detector}
}

// Detect implementa a interface shoplifting.ObjectDetector
func (adapter *YOLODetectorAdapter) Detect(img gocv.Mat) []shoplifting.DetectionResult {
	// Chama o detector original
	originalResults := adapter.detector.Detect(img)

	// Converte para o tipo do package shoplifting
	var results []shoplifting.DetectionResult
	for _, orig := range originalResults {
		results = append(results, shoplifting.DetectionResult{
			ClassID:    orig.ClassID,
			Confidence: orig.Confidence,
			Box:        orig.Box,
			Label:      orig.Label,
		})
	}

	return results
}

// NewYOLODetector cria um novo detector YOLO
func NewYOLODetector(cfg *config.Config) (*YOLODetector, error) {
	// Carrega a rede neural
	net := gocv.ReadNetFromONNX(cfg.ObjectDetectionModel)
	if net.Empty() {
		return nil, fmt.Errorf("erro ao carregar modelo: %s", cfg.ObjectDetectionModel)
	}

	// Configura backend e target
	if err := net.SetPreferableBackend(gocv.NetBackendDefault); err != nil {
		return nil, fmt.Errorf("erro ao configurar backend: %v", err)
	}
	if err := net.SetPreferableTarget(gocv.NetTargetCPU); err != nil {
		return nil, fmt.Errorf("erro ao configurar target: %v", err)
	}

	// Carrega nomes das classes
	classNames, err := loadClassNames(cfg.ClassNamesFile)
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar classes: %v", err)
	}

	return &YOLODetector{
		net:        net,
		classNames: classNames,
		config:     cfg,
	}, nil
}

// Close libera os recursos do detector
func (d *YOLODetector) Close() {
	d.net.Close()
}

// Detect executa detecção em uma imagem
func (d *YOLODetector) Detect(img gocv.Mat) []DetectionResult {
	// Prepara entrada para o modelo
	blob := gocv.BlobFromImage(img, 1.0/255.0, image.Pt(d.config.InputSize, d.config.InputSize),
		gocv.NewScalar(0, 0, 0, 0), true, false)
	defer blob.Close()

	// Executa inferência
	d.net.SetInput(blob, "")
	output := d.net.Forward("")
	defer output.Close()

	// Processa detecções
	return d.processDetections(output, img.Cols(), img.Rows())
}

// processDetections converte saída do modelo em detecções válidas
func (d *YOLODetector) processDetections(output gocv.Mat, frameWidth, frameHeight int) []DetectionResult {
	data, _ := output.DataPtrFloat32()

	var rawDetections []DetectionResult
	scaleX := float32(frameWidth) / float32(d.config.InputSize)
	scaleY := float32(frameHeight) / float32(d.config.InputSize)

	// Processa todas as detecções
	for i := 0; i < d.config.NumDetections; i++ {
		detection := d.parseDetection(data, i, scaleX, scaleY, frameWidth, frameHeight)
		if detection != nil {
			rawDetections = append(rawDetections, *detection)
		}
	}

	// Aplica Non-Maximum Suppression
	return d.applyNMS(rawDetections)
}

// parseDetection extrai uma detecção individual dos dados brutos
func (d *YOLODetector) parseDetection(data []float32, index int, scaleX, scaleY float32, frameWidth, frameHeight int) *DetectionResult {
	// Extrai coordenadas (formato transposto)
	centerX := data[0*d.config.NumDetections + index]
	centerY := data[1*d.config.NumDetections + index]
	width := data[2*d.config.NumDetections + index]
	height := data[3*d.config.NumDetections + index]

	// Encontra classe com maior confiança
	classID, confidence := d.findBestClass(data, index)

	// Valida detecção
	if classID < 0 || classID > d.config.MaxValidClassID || confidence < d.config.ConfidenceThreshold {
		return nil
	}

	// Converte coordenadas para pixels
	box := d.convertToPixelCoordinates(centerX, centerY, width, height,
		scaleX, scaleY, frameWidth, frameHeight)

	// Filtra objetos muito pequenos
	if box.Dx() < d.config.MinObjectSize || box.Dy() < d.config.MinObjectSize {
		return nil
	}

	// Cria label
	label := fmt.Sprintf("%s: %.2f", d.classNames[classID], confidence)

	return &DetectionResult{
		ClassID:    classID,
		Confidence: confidence,
		Box:        box,
		Label:      label,
	}
}

// findBestClass encontra a classe com maior confiança
func (d *YOLODetector) findBestClass(data []float32, index int) (int, float32) {
	var bestClassID int
	var maxScore float32

	// Verifica se temos dados suficientes
	dataLength := len(data)
	maxIndex := (d.config.NumAttributes - 1) * d.config.NumDetections + index

	if maxIndex >= dataLength {
		return 0, 0.0
	}

	for j := 4; j < d.config.NumAttributes; j++ {
		dataIndex := j*d.config.NumDetections + index
		if dataIndex < dataLength {
			score := data[dataIndex]
			if score > maxScore {
				maxScore = score
				bestClassID = j - 4 // classes 0-364
			}
		}
	}

	return bestClassID, maxScore
}

// convertToPixelCoordinates converte coordenadas normalizadas para pixels
func (d *YOLODetector) convertToPixelCoordinates(centerX, centerY, width, height, scaleX, scaleY float32, frameWidth, frameHeight int) image.Rectangle {
	// Converte para coordenadas de pixel
	pixelCenterX := int(centerX * scaleX)
	pixelCenterY := int(centerY * scaleY)
	pixelWidth := int(width * scaleX)
	pixelHeight := int(height * scaleY)

	// Calcula coordenadas da caixa
	left := pixelCenterX - pixelWidth/2
	top := pixelCenterY - pixelHeight/2

	// Garante que está dentro dos limites da imagem
	left = max(0, left)
	top = max(0, top)
	right := min(frameWidth, left+pixelWidth)
	bottom := min(frameHeight, top+pixelHeight)

	return image.Rect(left, top, right, bottom)
}

// applyNMS aplica Non-Maximum Suppression para remover detecções duplicadas
func (d *YOLODetector) applyNMS(detections []DetectionResult) []DetectionResult {
	if len(detections) == 0 {
		return detections
	}

	// Prepara dados para NMS
	var boxes []image.Rectangle
	var confidences []float32

	for _, det := range detections {
		boxes = append(boxes, det.Box)
		confidences = append(confidences, det.Confidence)
	}

	// Aplica NMS
	indices := gocv.NMSBoxes(boxes, confidences, d.config.ConfidenceThreshold, d.config.NMSThreshold)

	// Retorna apenas detecções válidas
	var result []DetectionResult
	for _, idx := range indices {
		result = append(result, detections[idx])
	}

	return result
}

// DrawDetections desenha as detecções na imagem
func DrawDetections(img *gocv.Mat, detections []DetectionResult) {
	for _, det := range detections {
		// Gera cor única para a classe
		color := generateClassColor(det.ClassID)

		// Desenha retângulo e label
		gocv.Rectangle(img, det.Box, color, 3)
		gocv.PutText(img, det.Label,
			image.Pt(det.Box.Min.X, det.Box.Min.Y-5),
			gocv.FontHersheySimplex, 0.7, color, 2)
	}
}

// generateClassColor gera uma cor única para cada classe
func generateClassColor(classID int) color.RGBA {
	h := float64(classID*137%360) / 360.0 // Hue baseado no ID
	s := 0.7                              // Saturação fixa
	v := 0.9                              // Brilho fixo

	r, g, b := hsvToRGB(h, s, v)
	return color.RGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
}

// hsvToRGB converte HSV para RGB
func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 1.0/6:
		r, g, b = c, x, 0
	case h < 2.0/6:
		r, g, b = x, c, 0
	case h < 3.0/6:
		r, g, b = 0, c, x
	case h < 4.0/6:
		r, g, b = 0, x, c
	case h < 5.0/6:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return r + m, g + m, b + m
}

// loadClassNames carrega os nomes das classes do arquivo
func loadClassNames(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// setupCamera inicializa e configura a câmera (com fallback para múltiplos índices)
func setupCamera() (*gocv.VideoCapture, error) {
	// Tenta diferentes índices de câmera
	for i := 0; i < 4; i++ {
		fmt.Printf("🔍 Tentando câmera índice %d...\n", i)
		webcam, err := gocv.VideoCaptureDevice(i)
		if err != nil {
			fmt.Printf("❌ Câmera %d: %v\n", i, err)
			continue
		}

		// Testa se consegue capturar um frame
		testImg := gocv.NewMat()
		if ok := webcam.Read(&testImg); ok && !testImg.Empty() {
			testImg.Close()
			fmt.Printf("✅ Câmera %d funcionando!\n", i)
			return webcam, nil
		}

		testImg.Close()
		webcam.Close()
		fmt.Printf("⚠️  Câmera %d não consegue capturar frames\n", i)
	}

	return nil, fmt.Errorf("nenhuma câmera funcional encontrada (testados índices 0-3)")
}

// setupWindow cria e configura a janela de visualização
func setupWindow(title string) *gocv.Window {
	window := gocv.NewWindow(title)
	window.SetWindowProperty(gocv.WindowPropertyFullscreen, gocv.WindowNormal)
	return window
}

// handleInput verifica input do usuário para sair
func handleInput(window *gocv.Window) bool {
	key := window.WaitKey(30)

	// ESC ou Q para sair
	if key == 27 || key == 'q' || key == 'Q' {
		return true
	}

	// Verifica se janela foi fechada
	if !window.IsOpen() {
		return true
	}

	return false
}

// Funções utilitárias
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	// Configuração para shoplifting detection
	appConfig = config.DefaultConfig()
	runShopliftingDetection()
}

// runShopliftingDetection executa detecção de shoplifting
func runShopliftingDetection() {
	// Inicializa detector de objetos base
	objectDetector, err := NewYOLODetector(appConfig)
	if err != nil {
		fmt.Printf("❌ Erro ao inicializar detector de objetos: %v\n", err)
		os.Exit(1)
	}
	defer objectDetector.Close()

	// Cria adapter para o detector YOLO
	detectorAdapter := NewYOLODetectorAdapter(objectDetector)

	// Inicializa detector de shoplifting integrado
	shopliftingDetector, err := shoplifting.NewShopliftingDetector(detectorAdapter, appConfig)
	if err != nil {
		fmt.Printf("❌ Erro ao inicializar detector de shoplifting: %v\n", err)
		os.Exit(1)
	}
	defer shopliftingDetector.Close()

	// Configura câmera
	webcam, err := setupCamera()
	if err != nil {
		fmt.Printf("❌ Erro na câmera: %v\n", err)
		os.Exit(1)
	}
	defer webcam.Close()

	// Configura janela
	window := setupWindow(appConfig.WindowName)
	defer window.Close()

	// Prepara buffer para frames
	img := gocv.NewMat()
	defer img.Close()

	// Informações iniciais
	fmt.Println("🛡️  SHOPLIFTING DETECTOR ATIVO")
	fmt.Println("🤖 YOLO v11 + Pose Estimation")
	fmt.Println("👥 Detecta pessoas e comportamentos suspeitos")
	fmt.Println("🚨 Alertas em tempo real para:")
	fmt.Println("   • Pessoas vagueando por muito tempo")
	fmt.Println("   • Posições suspeitas (agachado, escondido)")
	fmt.Println("   • Proximidade com itens valiosos")
	fmt.Println("   • Movimentos de ocultação")
	fmt.Println("📱 Pressione ESC ou Q para sair")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	frameCount := 0
	alertCount := 0

	// Loop principal de detecção
	for {
		// Captura frame
		if ok := webcam.Read(&img); !ok {
			fmt.Println("❌ Erro ao ler da webcam")
			break
		}

		if img.Empty() {
			continue
		}

		frameCount++

		// Executa detecção de shoplifting
		detections, suspiciousBehaviors := shopliftingDetector.DetectShoplifting(img)

		// Conta alertas
		if len(suspiciousBehaviors) > 0 {
			alertCount += len(suspiciousBehaviors)

			// Log dos comportamentos suspeitos
			for _, behavior := range suspiciousBehaviors {
				fmt.Printf("🚨 ALERTA: %s (Confiança: %.1f%%) - %s\n",
					behavior.Type, behavior.Confidence*100, behavior.Description)
			}
		}

		// Desenha resultados na imagem
		shoplifting.DrawShopliftingDetections(&img, detections, suspiciousBehaviors)

		// Desenha poses se disponíveis (debug visual)
		if len(detections) > 0 {
			// Obtém poses da última detecção para visualização
			poses := shopliftingDetector.GetLastPoses()
			shoplifting.DrawPoseKeypoints(&img, poses)
		}

		// Adiciona informações de status na imagem
		addStatusInfo(&img, frameCount, len(detections), len(suspiciousBehaviors), alertCount)

		// Mostra na janela
		window.IMShow(img)

		// Verifica input do usuário
		if handleInput(window) {
			break
		}
	}

	// Estatísticas finais
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 ESTATÍSTICAS FINAIS:\n")
	fmt.Printf("   • Frames processados: %d\n", frameCount)
	fmt.Printf("   • Total de alertas: %d\n", alertCount)
	fmt.Printf("   • Taxa de alertas: %.2f%%\n", float64(alertCount)/float64(frameCount)*100)
	fmt.Println("👋 Detector de shoplifting encerrado")
}

// addStatusInfo adiciona informações de status na imagem
func addStatusInfo(img *gocv.Mat, frameCount, detectionCount, alertCount, totalAlerts int) {
	// Painel de informações no topo
	statusText := fmt.Sprintf("Frame: %d | Deteccoes: %d | Alertas Ativos: %d | Total: %d",
		frameCount, detectionCount, alertCount, totalAlerts)

	// Fundo semi-transparente para o texto
	gocv.Rectangle(img,
		image.Rect(0, 0, img.Cols(), 60),
		color.RGBA{0, 0, 0, 180}, -1)

	// Texto de status
	gocv.PutText(img, statusText,
		image.Pt(10, 25),
		gocv.FontHersheySimplex, 0.6,
		color.RGBA{255, 255, 255, 255}, 2)

	// Indicador de status (verde = normal, vermelho = alerta)
	statusColor := color.RGBA{0, 255, 0, 255} // Verde
	statusIcon := "🟢 NORMAL"

	if alertCount > 0 {
		statusColor = color.RGBA{255, 0, 0, 255} // Vermelho
		statusIcon = "🔴 ALERTA"
	}

	gocv.PutText(img, statusIcon,
		image.Pt(10, 50),
		gocv.FontHersheySimplex, 0.6,
		statusColor, 2)

	// Timestamp
	currentTime := time.Now().Format("15:04:05")
	gocv.PutText(img, currentTime,
		image.Pt(img.Cols()-100, 25),
		gocv.FontHersheySimplex, 0.6,
		color.RGBA{255, 255, 255, 255}, 2)
}