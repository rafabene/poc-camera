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

const (
	// YOLOv11n Object365 (365 classes - dataset muito mais abrangente)
	modelWeights        = "models/yolo11n_object365.onnx"
	classNamesFile      = "models/object365.names"
	confidenceThreshold = 0.5  // Threshold balanceado para detectar objetos
	nmsThreshold        = 0.4  // NMS balanceado
	minObjectSize       = 50   // Objetos menores permitidos
	maxValidClassID     = 364  // Object365 dataset tem classes 0-364 (365 classes total)
)

// generateClassColor gera uma cor única para cada classe baseada no ID da classe
func generateClassColor(classID int) color.RGBA {
	// Usa o ID da classe para gerar uma cor consistente
	h := float64(classID*137%360) / 360.0 // Hue baseado no ID da classe
	s := 0.7                              // Saturação fixa
	v := 0.9                              // Valor (brilho) fixo

	// Converte HSV para RGB
	r, g, b := hsvToRGB(h, s, v)
	return color.RGBA{uint8(r * 255), uint8(g * 255), uint8(b * 255), 255}
}

// hsvToRGB converte valores HSV para RGB
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

func main() {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		fmt.Printf("Erro ao abrir a webcam: %v\n", err)
		os.Exit(1)
	}
	defer webcam.Close()

	// Cria a janela
	windowName := "POC Camera"
	window := gocv.NewWindow(windowName)
	defer window.Close()

	// Tenta habilitar os controles da janela (pode não funcionar em todos os sistemas)
	window.SetWindowProperty(gocv.WindowPropertyFullscreen, gocv.WindowNormal)

	img := gocv.NewMat()
	defer img.Close()

	// Carrega a rede neural YOLOv11 (ONNX)
	net := gocv.ReadNetFromONNX(modelWeights)

	if net.Empty() {
		fmt.Printf("Erro ao ler a rede neural: %s\n", modelWeights)
		os.Exit(1)
	}
	defer net.Close()

	if err := net.SetPreferableBackend(gocv.NetBackendDefault); err != nil {
		fmt.Printf("Erro ao definir o backend: %v\n", err)
	}
	if err := net.SetPreferableTarget(gocv.NetTargetCPU); err != nil {
		fmt.Printf("Erro ao definir o alvo: %v\n", err)
	}

	classNames, err := readClassNames(classNamesFile)
	if err != nil {
		fmt.Printf("Erro ao ler os nomes das classes: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Usando YOLOv11n Object365 (365 classes - dataset muito mais abrangente)")
	fmt.Println("Pressione ESC ou Q para sair (ou use Cmd+Q para forçar o fechamento).")

	for {
		if ok := webcam.Read(&img); !ok {
			fmt.Println("Não foi possível ler o quadro da webcam.")
			break
		}
		if img.Empty() {
			continue
		}

		// YOLOv11 usa entrada 640x640
		inputSize := image.Pt(640, 640)
		blob := gocv.BlobFromImage(img, 1.0/255.0, inputSize, gocv.NewScalar(0, 0, 0, 0), true, false)

		net.SetInput(blob, "")

		blob.Close()

		// Executa inferência do YOLOv11
		output := net.Forward("")
		outputs := []gocv.Mat{output}

		// Processa detecções do YOLOv11
		performDetection(img, outputs, classNames)

		for i := range outputs {
			outputs[i].Close()
		}

		window.IMShow(img)

		// Verifica teclas pressionadas
		key := window.WaitKey(30)
		if key == 27 {  // ESC
			break
		}
		if key == 'q' || key == 'Q' {  // Q para quit
			break
		}

		// Verifica se a janela ainda existe (detecção alternativa de fechamento)
		if !window.IsOpen() {
			break
		}
	}
}

func performDetection(frame gocv.Mat, results []gocv.Mat, classNames []string) {
	var classIDs []int
	var confidences []float32
	var boxes []image.Rectangle

	for _, result := range results {
		data, _ := result.DataPtrFloat32()

		// YOLOv11 Object365 formato: [369, 8400] onde data = 3,099,600 elementos
		// Formato transposto: primeiro todos os x, depois todos os y, etc.
		// data[0...8399] = x coords, data[8400...16799] = y coords, etc.

		// Processa todas as 8400 detecções do formato transposto
		for i := 0; i < 8400; i++ {
			// YOLOv11 formato transposto: data[atributo * 8400 + detecção]
			centerX := data[0*8400 + i]  // x
			centerY := data[1*8400 + i]  // y
			width := data[2*8400 + i]    // w
			height := data[3*8400 + i]   // h

			// Classes: atributos 4-368 (365 classes Object365)
			var classID int
			var maxClassScore float32
			for j := 4; j < 369; j++ {
				classScore := data[j*8400 + i]
				if classScore > maxClassScore {
					maxClassScore = classScore
					classID = j - 4  // classID correto: 0-364
				}
			}

			finalConfidence := maxClassScore

			// Filtros básicos
			if classID < 0 || classID > maxValidClassID {
				continue
			}

			if finalConfidence > float32(confidenceThreshold) {
				frameWidth := frame.Cols()
				frameHeight := frame.Rows()

				// YOLOv11 retorna coordenadas no espaço da imagem de entrada (640x640)
				// Precisa mapear de 640x640 para as dimensões reais do frame
				scaleX := float32(frameWidth) / 640.0
				scaleY := float32(frameHeight) / 640.0

				pixelCenterX := int(centerX * scaleX)
				pixelCenterY := int(centerY * scaleY)
				pixelWidth := int(width * scaleX)
				pixelHeight := int(height * scaleY)

				left := pixelCenterX - pixelWidth/2
				top := pixelCenterY - pixelHeight/2

				// Garantir que as coordenadas estão dentro do frame
				if left < 0 { left = 0 }
				if top < 0 { top = 0 }
				if left+pixelWidth > frameWidth { pixelWidth = frameWidth - left }
				if top+pixelHeight > frameHeight { pixelHeight = frameHeight - top }

				// Filtrar objetos muito pequenos (reduz ruído)
				if pixelWidth < minObjectSize || pixelHeight < minObjectSize {
					continue
				}

				classIDs = append(classIDs, classID)
				confidences = append(confidences, finalConfidence)
				boxes = append(boxes, image.Rect(left, top, left+pixelWidth, top+pixelHeight))
			}
		}
	}

	if len(boxes) == 0 {
		return
	}

	indices := gocv.NMSBoxes(boxes, confidences, confidenceThreshold, nmsThreshold)

	for _, idx := range indices {
		box := boxes[idx]
		classID := classIDs[idx]

		// Proteção adicional: verificar se classID é válido para o array classNames
		if classID < 0 || classID >= len(classNames) {
			continue
		}

		label := fmt.Sprintf("%s: %.2f", classNames[classID], confidences[idx])

		// Gera uma cor única para cada classe de objeto
		objColor := generateClassColor(classID)

		// Desenha um quadrado colorido para o objeto detectado
		gocv.Rectangle(&frame, box, objColor, 3)
		gocv.PutText(&frame, label, image.Pt(box.Min.X, box.Min.Y-5), gocv.FontHersheySimplex, 0.7, objColor, 2)
	}
}

func readClassNames(filename string) ([]string, error) {
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
