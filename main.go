package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"os"

	"gocv.io/x/gocv"
)

const (
	modelCfg            = "models/yolov4-tiny.cfg"
	modelWeights        = "models/yolov4-tiny.weights"
	classNamesFile      = "models/coco.names"
	confidenceThreshold = 0.5
	nmsThreshold        = 0.4
)

func main() {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		fmt.Printf("Erro ao abrir a webcam: %v\n", err)
		os.Exit(1)
	}
	defer webcam.Close()

	window := gocv.NewWindow("POC Camera")
	defer window.Close()

	img := gocv.NewMat()
	defer img.Close()

	net := gocv.ReadNet(modelWeights, modelCfg)
	if net.Empty() {
		fmt.Printf("Erro ao ler a rede neural: %s, %s\n", modelWeights, modelCfg)
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

	fmt.Println("Pressione ESC ou feche a janela para sair.")

	for {
		if ok := webcam.Read(&img); !ok {
			fmt.Println("Não foi possível ler o quadro da webcam.")
			break
		}
		if img.Empty() {
			continue
		}

		blob := gocv.BlobFromImage(img, 1.0/255.0, image.Pt(416, 416), gocv.NewScalar(0, 0, 0, 0), true, false)

		net.SetInput(blob, "")

		blob.Close()

		layerNames := net.GetLayerNames()
		outputLayerIDs := net.GetUnconnectedOutLayers()
		var outputLayers []string
		for _, id := range outputLayerIDs {
			outputLayers = append(outputLayers, layerNames[id-1])
		}
		outputs := net.ForwardLayers(outputLayers)

		performDetection(img, outputs, classNames)

		for i := range outputs {
			outputs[i].Close()
		}

		window.IMShow(img)

		if window.WaitKey(30) == 27 {
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

		rows := result.Rows()

		cols := result.Cols()

		for i := 0; i < rows; i++ {
			row := data[i*cols : (i+1)*cols]
			scores := row[5:]
			var classID int
			var confidence float32
			for i, score := range scores {
				if score > confidence {
					confidence = score
					classID = i
				}
			}

			if confidence > confidenceThreshold {
				centerX := int(row[0] * float32(frame.Cols()))
				centerY := int(row[1] * float32(frame.Rows()))
				width := int(row[2] * float32(frame.Cols()))
				height := int(row[3] * float32(frame.Rows()))
				left := centerX - width/2
				top := centerY - height/2

				classIDs = append(classIDs, classID)
				confidences = append(confidences, confidence)
				boxes = append(boxes, image.Rect(left, top, left+width, top+height))
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
		label := fmt.Sprintf("%s: %.2f", classNames[classID], confidences[idx])

		blue := color.RGBA{0, 0, 255, 0}
		gocv.Rectangle(&frame, box, blue, 2)
		gocv.PutText(&frame, label, image.Pt(box.Min.X, box.Min.Y-5), gocv.FontHersheySimplex, 0.5, blue, 2)
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
