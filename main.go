package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	"gocv.io/x/gocv"
)

// Configuração do modelo YOLOv11 Object365
const (
	modelWeights        = "models/yolo11n_object365.onnx"
	classNamesFile      = "models/object365.names"
	confidenceThreshold = 0.25 // Threshold para detectar objetos
	nmsThreshold        = 0.4  // NMS padrão
	minObjectSize       = 20   // Tamanho mínimo dos objetos
	maxValidClassID     = 364  // Object365: 365 classes (0-364)

	// Configurações da aplicação
	windowName    = "POC Camera - YOLOv11 Object365 Detection"
	inputSize     = 640    // Tamanho da entrada do modelo
	numDetections = 8400   // Número de detecções do YOLOv11
	numAttributes = 369    // 4 coordenadas + 365 classes Object365
)

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
}

// NewYOLODetector cria um novo detector YOLO
func NewYOLODetector() (*YOLODetector, error) {
	// Carrega a rede neural
	net := gocv.ReadNetFromONNX(modelWeights)
	if net.Empty() {
		return nil, fmt.Errorf("erro ao carregar modelo: %s", modelWeights)
	}

	// Configura backend e target
	if err := net.SetPreferableBackend(gocv.NetBackendDefault); err != nil {
		return nil, fmt.Errorf("erro ao configurar backend: %v", err)
	}
	if err := net.SetPreferableTarget(gocv.NetTargetCPU); err != nil {
		return nil, fmt.Errorf("erro ao configurar target: %v", err)
	}

	// Carrega nomes das classes
	classNames, err := loadClassNames(classNamesFile)
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar classes: %v", err)
	}

	return &YOLODetector{
		net:        net,
		classNames: classNames,
	}, nil
}

// Close libera os recursos do detector
func (d *YOLODetector) Close() {
	d.net.Close()
}

// Detect executa detecção em uma imagem
func (d *YOLODetector) Detect(img gocv.Mat) []DetectionResult {
	// Prepara entrada para o modelo
	blob := gocv.BlobFromImage(img, 1.0/255.0, image.Pt(inputSize, inputSize),
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
	scaleX := float32(frameWidth) / float32(inputSize)
	scaleY := float32(frameHeight) / float32(inputSize)

	// Processa todas as detecções
	for i := 0; i < numDetections; i++ {
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
	centerX := data[0*numDetections + index]
	centerY := data[1*numDetections + index]
	width := data[2*numDetections + index]
	height := data[3*numDetections + index]

	// Encontra classe com maior confiança
	classID, confidence := d.findBestClass(data, index)

	// Valida detecção
	if classID < 0 || classID > maxValidClassID || confidence < confidenceThreshold {
		return nil
	}

	// Converte coordenadas para pixels
	box := d.convertToPixelCoordinates(centerX, centerY, width, height,
		scaleX, scaleY, frameWidth, frameHeight)

	// Filtra objetos muito pequenos
	if box.Dx() < minObjectSize || box.Dy() < minObjectSize {
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
	maxIndex := (numAttributes - 1) * numDetections + index

	if maxIndex >= dataLength {
		// Se não temos dados suficientes, retorna valores padrão
		return 0, 0.0
	}

	for j := 4; j < numAttributes; j++ {
		dataIndex := j*numDetections + index
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
	indices := gocv.NMSBoxes(boxes, confidences, confidenceThreshold, nmsThreshold)

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

// setupCamera inicializa e configura a câmera
func setupCamera() (*gocv.VideoCapture, error) {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir webcam: %v", err)
	}
	return webcam, nil
}

// setupWindow cria e configura a janela de visualização
func setupWindow() *gocv.Window {
	window := gocv.NewWindow(windowName)
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
	// Inicializa detector
	detector, err := NewYOLODetector()
	if err != nil {
		fmt.Printf("Erro ao inicializar detector: %v\n", err)
		os.Exit(1)
	}
	defer detector.Close()

	// Configura câmera
	webcam, err := setupCamera()
	if err != nil {
		fmt.Printf("Erro na câmera: %v\n", err)
		os.Exit(1)
	}
	defer webcam.Close()

	// Configura janela
	window := setupWindow()
	defer window.Close()

	// Prepara buffer para frames
	img := gocv.NewMat()
	defer img.Close()

	fmt.Println("🔍 YOLOv11 Object365 Detection - Detecção de 365 objetos")
	fmt.Println("🌍 Detecta pessoas, veículos, móveis, comida, animais e muito mais")
	fmt.Println("📱 Pressione ESC ou Q para sair")

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

		// Executa detecção
		detections := detector.Detect(img)

		// Desenha resultados
		DrawDetections(&img, detections)

		// Mostra na janela
		window.IMShow(img)

		// Verifica se deve sair
		if handleInput(window) {
			break
		}
	}

	fmt.Println("👋 Aplicação encerrada")
}