package shoplifting

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"time"

	"gocv.io/x/gocv"
	"poc-camera/config"
)

// PoseKeypoint representa um ponto chave do corpo
type PoseKeypoint struct {
	X          float32
	Y          float32
	Confidence float32
}

// PersonPose representa a pose de uma pessoa detectada
type PersonPose struct {
	Keypoints  []PoseKeypoint
	Confidence float32
	BoundingBox image.Rectangle
}

// TrackedPerson representa uma pessoa sendo rastreada ao longo do tempo
type TrackedPerson struct {
	ID              int
	LastSeen        time.Time
	Positions       []image.Point
	Poses           []PersonPose
	LoiteringTime   time.Duration
	SuspiciousCount int
	FirstSeen       time.Time
	LastSuspiciousMovement time.Time // Para cooldown
}

// SuspiciousBehavior representa um comportamento suspeito detectado
type SuspiciousBehavior struct {
	Type        string
	Confidence  float32
	Description string
	PersonID    int
	Location    image.Point
}

// DetectionResult representa uma detecção de objeto (definido aqui para independência)
type DetectionResult struct {
	ClassID    int
	Confidence float32
	Box        image.Rectangle
	Label      string
}

// ObjectDetector interface para detector de objetos
type ObjectDetector interface {
	Detect(img gocv.Mat) []DetectionResult
}

// ShopliftingDetector gerencia detecção de shoplifting
type ShopliftingDetector struct {
	objectDetector ObjectDetector
	poseNet        gocv.Net
	poseEnabled    bool
	trackedPeople  map[int]*TrackedPerson
	nextPersonID   int
	config         *config.Config
	valuableItems  map[int]string
	frameCount     int
	lastPoseFrame  int
	lastPoses      []PersonPose
}

// NewShopliftingDetector cria um novo detector de shoplifting
func NewShopliftingDetector(objectDetector ObjectDetector, cfg *config.Config) (*ShopliftingDetector, error) {
	var poseNet gocv.Net
	var poseEnabled bool

	// Tenta carregar modelo YOLO pose (com fallback inteligente)
	if poseModelExists(cfg.PoseEstimationModel) {
		net, err := loadPoseModelWithFallback(cfg.PoseEstimationModel)
		if err == nil {
			poseNet = net
			poseEnabled = true
			fmt.Println("✅ Pose estimation habilitado (YOLO v11)")
			fmt.Println("🎯 Modelo: YOLO v11 Pose (17 keypoints COCO)")
		} else {
			fmt.Printf("⚠️  Erro ao carregar pose model: %v\n", err)
		}
	}

	if !poseEnabled {
		fmt.Println("⚠️  Pose estimation desabilitado")
		fmt.Println("✅ Sistema funcionando com:")
		fmt.Println("   • Detecção de objetos (365 classes)")
		fmt.Println("   • Tracking de pessoas")
		fmt.Println("   • Detecção de loitering (tempo)")
		fmt.Println("   • Proximidade com itens valiosos")
		fmt.Println("   • Análise comportamental baseada em movimento")
	}

	return &ShopliftingDetector{
		objectDetector: objectDetector,
		poseNet:        poseNet,
		poseEnabled:    poseEnabled,
		trackedPeople:  make(map[int]*TrackedPerson),
		nextPersonID:   1,
		config:         cfg,
		valuableItems:  config.GetValuableItems(),
	}, nil
}

// poseModelExists verifica se o modelo de pose existe
func poseModelExists(modelPath string) bool {
	if modelPath == "" {
		return false
	}
	if _, err := os.Stat(modelPath); err != nil {
		return false
	}
	return true
}

// loadPoseModelWithFallback carrega modelo YOLO pose com estratégias de fallback
func loadPoseModelWithFallback(modelPath string) (gocv.Net, error) {
	var net gocv.Net

	// Estratégia 1: Carregamento direto
	net = gocv.ReadNetFromONNX(modelPath)
	if !net.Empty() {
		// Tenta configurar backend - se falhar, continua assim mesmo
		net.SetPreferableBackend(gocv.NetBackendDefault)
		net.SetPreferableTarget(gocv.NetTargetCPU)

		// Testa inferência com imagem dummy para verificar compatibilidade
		if testPoseInference(net) {
			return net, nil
		}
		net.Close()
	}

	return gocv.Net{}, fmt.Errorf("não foi possível carregar modelo pose com OpenCV atual")
}

// testPoseInference testa se a inferência funciona com uma imagem dummy
func testPoseInference(net gocv.Net) bool {
	// Cria imagem dummy 640x640
	dummyImg := gocv.NewMatWithSize(640, 640, gocv.MatTypeCV8UC3)
	defer dummyImg.Close()

	// Tenta criar blob
	blob := gocv.BlobFromImage(dummyImg, 1.0/255.0, image.Pt(640, 640),
		gocv.NewScalar(0, 0, 0, 0), true, false)
	defer blob.Close()

	// Tenta fazer inferência básica
	defer func() {
		if r := recover(); r != nil {
			// Se deu panic, modelo incompatível
		}
	}()

	net.SetInput(blob, "")
	output := net.Forward("")
	defer output.Close()

	return !output.Empty()
}

// Close libera recursos do detector
func (sd *ShopliftingDetector) Close() {
	if sd.poseEnabled && !sd.poseNet.Empty() {
		sd.poseNet.Close()
	}
}

// GetLastPoses retorna as últimas poses detectadas para visualização
func (sd *ShopliftingDetector) GetLastPoses() []PersonPose {
	return sd.lastPoses
}

// DetectShoplifting executa detecção completa de shoplifting
func (sd *ShopliftingDetector) DetectShoplifting(img gocv.Mat) ([]DetectionResult, []SuspiciousBehavior) {
	// 1. Detecta objetos (incluindo pessoas)
	detections := sd.objectDetector.Detect(img)

	// 2. Filtra pessoas e objetos valiosos
	people := sd.filterPeople(detections)
	valuableObjects := sd.filterValuableObjects(detections)

	// 3. Detecta poses para cada pessoa (se habilitado) - otimizado
	var poses []PersonPose
	sd.frameCount++
	if sd.poseEnabled && len(people) > 0 {
		// Processa pose apenas a cada 10 frames para performance
		if sd.frameCount-sd.lastPoseFrame >= 10 {
			poses = sd.detectPoses(img, people)
			sd.lastPoseFrame = sd.frameCount
			sd.lastPoses = poses // Armazena para visualização
		} else {
			// Usa poses da última detecção
			poses = sd.lastPoses
		}
	}

	// 4. Atualiza tracking de pessoas
	sd.updateTracking(people, poses)

	// 5. Analisa comportamentos suspeitos
	suspiciousBehaviors := sd.analyzeBehaviors(people, valuableObjects, poses)

	// 6. Remove pessoas que não são mais vistas
	sd.cleanupOldTracking()

	return detections, suspiciousBehaviors
}

// filterPeople filtra detecções que são pessoas (class ID 0 normalmente)
func (sd *ShopliftingDetector) filterPeople(detections []DetectionResult) []DetectionResult {
	var people []DetectionResult
	for _, det := range detections {
		// Assumindo que pessoa é class ID 0 no Object365
		if det.ClassID == 0 {
			people = append(people, det)
		}
	}
	return people
}

// filterValuableObjects filtra objetos valiosos
func (sd *ShopliftingDetector) filterValuableObjects(detections []DetectionResult) []DetectionResult {
	var valuable []DetectionResult
	for _, det := range detections {
		if _, isValuable := sd.valuableItems[det.ClassID]; isValuable {
			valuable = append(valuable, det)
		}
	}
	return valuable
}

// detectPoses detecta poses para pessoas identificadas usando YOLO (otimizado)
func (sd *ShopliftingDetector) detectPoses(img gocv.Mat, people []DetectionResult) []PersonPose {
	var poses []PersonPose

	// Verifica se pose está habilitado
	if !sd.poseEnabled || len(people) == 0 || sd.poseNet.Empty() {
		return poses
	}

	// Processa apenas a primeira pessoa por performance (pode expandir depois)
	person := people[0]

	// Verifica se a região é válida
	if person.Box.Dx() <= 0 || person.Box.Dy() <= 0 {
		return poses
	}

	// Extrai região da pessoa com padding
	roi := sd.extractPersonROI(img, person.Box)
	if roi.Empty() {
		return poses
	}
	defer roi.Close()

	// Detecta pose usando YOLO
	pose := sd.detectPoseWithYOLO(roi, person.Box)
	if pose.Confidence > 0.3 {
		poses = append(poses, pose)
	}

	return poses
}

// extractPersonROI extrai região da pessoa com padding para melhor detecção de pose
func (sd *ShopliftingDetector) extractPersonROI(img gocv.Mat, box image.Rectangle) gocv.Mat {
	// Adiciona padding de 10% para capturar pose completa
	padding := int(float32(max(box.Dx(), box.Dy())) * 0.1)

	left := max(0, box.Min.X-padding)
	top := max(0, box.Min.Y-padding)
	right := min(img.Cols(), box.Max.X+padding)
	bottom := min(img.Rows(), box.Max.Y+padding)

	paddedBox := image.Rect(left, top, right, bottom)
	return img.Region(paddedBox)
}

// detectPoseWithYOLO executa detecção de pose usando YOLO
func (sd *ShopliftingDetector) detectPoseWithYOLO(roi gocv.Mat, originalBox image.Rectangle) PersonPose {
	// Prepara entrada para YOLO pose
	blob := gocv.BlobFromImage(roi, 1.0/255.0, image.Pt(640, 640),
		gocv.NewScalar(0, 0, 0, 0), true, false)
	defer blob.Close()

	// Executa inferência
	sd.poseNet.SetInput(blob, "")
	output := sd.poseNet.Forward("")
	defer output.Close()

	// Processa saída do YOLO pose
	return sd.processPoseOutput(output, roi, originalBox)
}

// processPoseOutput converte saída do YOLO em keypoints
func (sd *ShopliftingDetector) processPoseOutput(output gocv.Mat, roi gocv.Mat, originalBox image.Rectangle) PersonPose {
	data, _ := output.DataPtrFloat32()

	// YOLO v11 pose output: (1, 56, 8400) flatten = 470400 elementos
	// Formato: 56 atributos × 8400 detecções possíveis
	// 56 = [x_center, y_center, width, height, obj_conf] + 51 keypoints (17×3)

	if len(data) != 470400 { // 56 × 8400
		fmt.Printf("❌ Formato inesperado: %d elementos\n", len(data))
		return PersonPose{Confidence: 0, BoundingBox: originalBox}
	}

	// Encontra a melhor detecção (maior obj_confidence no índice 4)
	bestDetectionIdx := 0
	bestObjConf := float32(0)

	for i := 0; i < 8400; i++ {
		objConfIdx := 4*8400 + i // obj_confidence está no índice 4
		if objConfIdx < len(data) {
			objConf := data[objConfIdx]
			if objConf > bestObjConf {
				bestObjConf = objConf
				bestDetectionIdx = i
			}
		}
	}

	if bestObjConf < 0.3 {
		return PersonPose{Confidence: 0, BoundingBox: originalBox}
	}


	var keypoints []PoseKeypoint
	var totalConfidence float32
	validPoints := 0

	// Extrai keypoints da melhor detecção
	for i := 0; i < 17; i++ {
		// Keypoints começam no atributo 5 (após bbox + obj_conf)
		xIdx := (5 + i*3) * 8400 + bestDetectionIdx
		yIdx := (5 + i*3 + 1) * 8400 + bestDetectionIdx
		visIdx := (5 + i*3 + 2) * 8400 + bestDetectionIdx

		if visIdx >= len(data) {
			break
		}

		x := data[xIdx]
		y := data[yIdx]
		vis := data[visIdx]

		// Coordenadas em pixels do input (640×640)
		// Escalar para a ROI
		roiX := x * float32(roi.Cols()) / 640.0
		roiY := y * float32(roi.Rows()) / 640.0

		// Converter para coordenadas absolutas
		absX := float32(originalBox.Min.X) + roiX
		absY := float32(originalBox.Min.Y) + roiY

		keypoints = append(keypoints, PoseKeypoint{
			X:          absX,
			Y:          absY,
			Confidence: vis,
		})

		if vis > 0.5 {
			totalConfidence += vis
			validPoints++
		}
	}

	avgConfidence := float32(0)
	if validPoints > 0 {
		avgConfidence = totalConfidence / float32(validPoints)
	}


	return PersonPose{
		Keypoints:   keypoints,
		Confidence:  avgConfidence,
		BoundingBox: originalBox,
	}
}


// updateTracking atualiza tracking de pessoas
func (sd *ShopliftingDetector) updateTracking(people []DetectionResult, poses []PersonPose) {
	currentTime := time.Now()

	// Associa detecções com pessoas rastreadas
	for i, person := range people {
		personCenter := image.Pt(
			person.Box.Min.X+person.Box.Dx()/2,
			person.Box.Min.Y+person.Box.Dy()/2,
		)

		// Procura pessoa existente próxima
		trackedID := sd.findNearestTrackedPerson(personCenter)

		if trackedID == -1 {
			// Nova pessoa
			trackedID = sd.nextPersonID
			sd.nextPersonID++

			sd.trackedPeople[trackedID] = &TrackedPerson{
				ID:        trackedID,
				FirstSeen: currentTime,
				LastSeen:  currentTime,
				Positions: []image.Point{personCenter},
			}
		}

		// Atualiza pessoa rastreada
		tracked := sd.trackedPeople[trackedID]
		tracked.LastSeen = currentTime
		tracked.Positions = append(tracked.Positions, personCenter)

		// Adiciona pose se disponível
		if i < len(poses) {
			tracked.Poses = append(tracked.Poses, poses[i])
		}

		// Calcula tempo de permanência
		tracked.LoiteringTime = currentTime.Sub(tracked.FirstSeen)

		// Limita histórico
		if len(tracked.Positions) > sd.config.MaxPoseHistory {
			tracked.Positions = tracked.Positions[1:]
		}
		if len(tracked.Poses) > sd.config.MaxPoseHistory {
			tracked.Poses = tracked.Poses[1:]
		}
	}
}

// findNearestTrackedPerson encontra pessoa rastreada mais próxima
func (sd *ShopliftingDetector) findNearestTrackedPerson(center image.Point) int {
	minDistance := float64(sd.config.ProximityThreshold)
	nearestID := -1

	for id, tracked := range sd.trackedPeople {
		if len(tracked.Positions) == 0 {
			continue
		}

		lastPos := tracked.Positions[len(tracked.Positions)-1]
		distance := math.Sqrt(float64((center.X-lastPos.X)*(center.X-lastPos.X) +
			(center.Y-lastPos.Y)*(center.Y-lastPos.Y)))

		if distance < minDistance {
			minDistance = distance
			nearestID = id
		}
	}

	return nearestID
}

// analyzeBehaviors analisa comportamentos suspeitos
func (sd *ShopliftingDetector) analyzeBehaviors(people []DetectionResult, valuableObjects []DetectionResult, poses []PersonPose) []SuspiciousBehavior {
	var behaviors []SuspiciousBehavior

	for id, tracked := range sd.trackedPeople {
		// Análise de tempo de permanência (loitering)
		if tracked.LoiteringTime.Seconds() > sd.config.LoiteringTimeThreshold {
			behaviors = append(behaviors, SuspiciousBehavior{
				Type:        "LOITERING",
				Confidence:  float32(math.Min(tracked.LoiteringTime.Seconds()/30.0, 1.0)),
				Description: fmt.Sprintf("Pessoa permanecendo na área por %.1f segundos", tracked.LoiteringTime.Seconds()),
				PersonID:    id,
				Location:    tracked.Positions[len(tracked.Positions)-1],
			})
		}

		// Análise de proximidade com objetos valiosos
		if len(tracked.Positions) > 0 {
			lastPos := tracked.Positions[len(tracked.Positions)-1]
			for _, valuable := range valuableObjects {
				valuableCenter := image.Pt(
					valuable.Box.Min.X+valuable.Box.Dx()/2,
					valuable.Box.Min.Y+valuable.Box.Dy()/2,
				)

				distance := math.Sqrt(float64((lastPos.X-valuableCenter.X)*(lastPos.X-valuableCenter.X) +
					(lastPos.Y-valuableCenter.Y)*(lastPos.Y-valuableCenter.Y)))

				if distance < sd.config.ProximityThreshold {
					behaviors = append(behaviors, SuspiciousBehavior{
						Type:        "VALUABLE_PROXIMITY",
						Confidence:  float32(1.0 - distance/sd.config.ProximityThreshold),
						Description: fmt.Sprintf("Próximo a %s", valuable.Label),
						PersonID:    id,
						Location:    lastPos,
					})
				}
			}
		}

		// Análise de movimento suspeito (apenas movimento recente com cooldown)
		if len(tracked.Positions) > 15 {
			currentTime := time.Now()
			// Cooldown de 8 segundos entre alertas de movimento suspeito
			if currentTime.Sub(tracked.LastSuspiciousMovement).Seconds() > 8.0 {
				// Analisa apenas as últimas 12 posições (movimento bem recente)
				recentPositions := tracked.Positions[len(tracked.Positions)-12:]

				movementScore := sd.analyzeSuspiciousMovement(recentPositions)

				// Threshold mais alto para evitar false positives
				if movementScore > 0.9 { // Era 0.8, agora 0.9
					behaviors = append(behaviors, SuspiciousBehavior{
						Type:        "SUSPICIOUS_MOVEMENT",
						Confidence:  movementScore,
						Description: "Padrão de movimento altamente suspeito detectado",
						PersonID:    id,
						Location:    tracked.Positions[len(tracked.Positions)-1],
					})
					// Atualiza timestamp do último alerta
					tracked.LastSuspiciousMovement = currentTime
				}
			}
		}

		// Análise de poses suspeitas (se disponível)
		if sd.poseEnabled && len(tracked.Poses) > 0 {
			lastPose := tracked.Poses[len(tracked.Poses)-1]
			suspiciousScore := sd.analyzeSuspiciousPose(lastPose)

			if suspiciousScore > sd.config.SuspiciousPoseThreshold {
				behaviors = append(behaviors, SuspiciousBehavior{
					Type:        "SUSPICIOUS_POSE",
					Confidence:  suspiciousScore,
					Description: "Postura suspeita detectada",
					PersonID:    id,
					Location:    tracked.Positions[len(tracked.Positions)-1],
				})
			}
		}
	}

	return behaviors
}

// analyzeSuspiciousPose analisa se uma pose é suspeita
func (sd *ShopliftingDetector) analyzeSuspiciousPose(pose PersonPose) float32 {
	if len(pose.Keypoints) < 17 {
		return 0
	}

	suspiciousScore := float32(0)

	// Keypoints COCO: 0=nose, 5=left_shoulder, 6=right_shoulder, 11=left_hip, 12=right_hip
	leftShoulder := pose.Keypoints[5]
	rightShoulder := pose.Keypoints[6]
	leftHip := pose.Keypoints[11]
	rightHip := pose.Keypoints[12]

	// Verifica se pessoa está agachada/escondida
	if leftShoulder.Confidence > 0.3 && rightShoulder.Confidence > 0.3 &&
		leftHip.Confidence > 0.3 && rightHip.Confidence > 0.3 {

		shoulderY := (leftShoulder.Y + rightShoulder.Y) / 2
		hipY := (leftHip.Y + rightHip.Y) / 2

		// Se ombros estão muito próximos dos quadris, pessoa pode estar agachada
		bodyHeight := math.Abs(float64(shoulderY - hipY))
		if bodyHeight < 50 { // pixels
			suspiciousScore += 0.4
		}
	}

	// Verifica posições de braços (possível ocultação)
	leftWrist := pose.Keypoints[9]
	rightWrist := pose.Keypoints[10]

	if leftWrist.Confidence > 0.3 && rightWrist.Confidence > 0.3 {
		// Se punhos estão próximos do corpo (possível ocultação)
		bodyCenter := (leftShoulder.X + rightShoulder.X) / 2
		leftDistance := math.Abs(float64(leftWrist.X - bodyCenter))
		rightDistance := math.Abs(float64(rightWrist.X - bodyCenter))

		if leftDistance < 30 && rightDistance < 30 {
			suspiciousScore += 0.3
		}
	}

	return suspiciousScore
}

// analyzeSuspiciousMovement analisa padrões de movimento suspeitos (otimizado)
func (sd *ShopliftingDetector) analyzeSuspiciousMovement(positions []image.Point) float32 {
	if len(positions) < 10 {
		return 0
	}

	suspiciousScore := float32(0)

	// Analisa movimento errático (muito ziguezague) - mais restritivo
	directionChanges := 0
	significantMoves := 0

	for i := 2; i < len(positions); i++ {
		prev := positions[i-2]
		curr := positions[i-1]
		next := positions[i]

		// Calcula vetores de direção
		vec1X := curr.X - prev.X
		vec1Y := curr.Y - prev.Y
		vec2X := next.X - curr.X
		vec2Y := next.Y - curr.Y

		// Só considera movimentos significativos (> 5 pixels)
		dist1 := math.Sqrt(float64(vec1X*vec1X + vec1Y*vec1Y))
		dist2 := math.Sqrt(float64(vec2X*vec2X + vec2Y*vec2Y))

		if dist1 > 5 && dist2 > 5 {
			significantMoves++
			// Produto escalar para verificar mudança de direção
			dotProduct := vec1X*vec2X + vec1Y*vec2Y
			if dotProduct < 0 { // Mudança de direção > 90 graus
				directionChanges++
			}
		}
	}

	// Se tem muitas mudanças de direção EM movimentos significativos, pode ser suspeito
	if significantMoves > 0 {
		changeRate := float32(directionChanges) / float32(significantMoves)
		if changeRate > 0.5 { // Mais de 50% de mudanças de direção em movimentos significativos
			suspiciousScore += changeRate * 0.6 // Peso reduzido
		}
	}

	// Analisa movimento circular/repetitivo
	recentPositions := positions[len(positions)-10:]
	var centerX, centerY float32
	for _, pos := range recentPositions {
		centerX += float32(pos.X)
		centerY += float32(pos.Y)
	}
	centerX /= float32(len(recentPositions))
	centerY /= float32(len(recentPositions))

	// Verifica se está circulando numa área pequena
	maxDistance := float32(0)
	for _, pos := range recentPositions {
		distance := math.Sqrt(float64((float32(pos.X)-centerX)*(float32(pos.X)-centerX) +
			(float32(pos.Y)-centerY)*(float32(pos.Y)-centerY)))
		if float32(distance) > maxDistance {
			maxDistance = float32(distance)
		}
	}

	// Se está se movendo numa área muito pequena, pode ser suspeito
	if maxDistance < 30 { // Raio menor que 30 pixels
		suspiciousScore += 0.4
	}

	// Analisa velocidade inconsistente
	speeds := make([]float32, 0)
	for i := 1; i < len(positions); i++ {
		prev := positions[i-1]
		curr := positions[i]
		distance := math.Sqrt(float64((curr.X-prev.X)*(curr.X-prev.X) +
			(curr.Y-prev.Y)*(curr.Y-prev.Y)))
		speeds = append(speeds, float32(distance))
	}

	// Calcula variação de velocidade
	if len(speeds) > 5 {
		var avgSpeed float32
		for _, speed := range speeds {
			avgSpeed += speed
		}
		avgSpeed /= float32(len(speeds))

		var variance float32
		for _, speed := range speeds {
			variance += (speed - avgSpeed) * (speed - avgSpeed)
		}
		variance /= float32(len(speeds))

		// Se a variação de velocidade é muito alta, pode ser suspeito
		if variance > 100 { // Velocidade muito inconsistente
			suspiciousScore += 0.3
		}
	}

	// Limita o score entre 0 e 1
	if suspiciousScore > 1.0 {
		suspiciousScore = 1.0
	}

	return suspiciousScore
}

// cleanupOldTracking remove pessoas que não são mais vistas
func (sd *ShopliftingDetector) cleanupOldTracking() {
	currentTime := time.Now()

	for id, tracked := range sd.trackedPeople {
		if currentTime.Sub(tracked.LastSeen).Seconds() > sd.config.TrackerTimeout {
			delete(sd.trackedPeople, id)
		}
	}
}

// DrawShopliftingDetections desenha detecções e alertas na imagem
func DrawShopliftingDetections(img *gocv.Mat, detections []DetectionResult, behaviors []SuspiciousBehavior) {
	// Desenha detecções normais
	for _, det := range detections {
		// Gera cor única para a classe
		color := generateClassColor(det.ClassID)

		// Desenha retângulo e label
		gocv.Rectangle(img, det.Box, color, 3)
		gocv.PutText(img, det.Label,
			image.Pt(det.Box.Min.X, det.Box.Min.Y-5),
			gocv.FontHersheySimplex, 0.7, color, 2)
	}

	// Desenha alertas de comportamento suspeito
	for _, behavior := range behaviors {
		alertColor := color.RGBA{255, 0, 0, 255} // Vermelho para alertas

		// Desenha círculo no local do alerta
		gocv.Circle(img, behavior.Location, 30, alertColor, 3)

		// Desenha texto do alerta
		alertText := fmt.Sprintf("%s (%.1f%%)", behavior.Type, behavior.Confidence*100)
		gocv.PutText(img, alertText,
			image.Pt(behavior.Location.X-50, behavior.Location.Y-40),
			gocv.FontHersheySimplex, 0.6, alertColor, 2)

		// Desenha descrição
		gocv.PutText(img, behavior.Description,
			image.Pt(behavior.Location.X-50, behavior.Location.Y-20),
			gocv.FontHersheySimplex, 0.4, alertColor, 1)
	}
}

// DrawPoseKeypoints desenha keypoints de pose na imagem
func DrawPoseKeypoints(img *gocv.Mat, poses []PersonPose) {
	poseColor := color.RGBA{0, 255, 255, 255} // Cyan para poses

	for _, pose := range poses {
		// Desenha todos os keypoints
		for i, kp := range pose.Keypoints {
			if kp.Confidence > 0.3 {
				// Desenha círculo para o keypoint
				gocv.Circle(img, image.Pt(int(kp.X), int(kp.Y)), 5, poseColor, -1)

				// Label do keypoint (opcional)
				if i < len(cocoKeypointNames) {
					gocv.PutText(img, fmt.Sprintf("%d", i),
						image.Pt(int(kp.X)+5, int(kp.Y)-5),
						gocv.FontHersheySimplex, 0.3, poseColor, 1)
				}
			}
		}

		// Desenha linhas conectando keypoints (esqueleto)
		drawPoseSkeleton(img, pose, poseColor)
	}
}

// cocoKeypointNames nomes dos keypoints COCO
var cocoKeypointNames = []string{
	"nose", "left_eye", "right_eye", "left_ear", "right_ear",
	"left_shoulder", "right_shoulder", "left_elbow", "right_elbow",
	"left_wrist", "right_wrist", "left_hip", "right_hip",
	"left_knee", "right_knee", "left_ankle", "right_ankle",
}

// drawPoseSkeleton desenha o esqueleto conectando keypoints
func drawPoseSkeleton(img *gocv.Mat, pose PersonPose, skeletonColor color.RGBA) {
	// Conexões COCO pose (pares de keypoints para desenhar linhas)
	connections := [][2]int{
		{5, 6}, {5, 7}, {7, 9}, {6, 8}, {8, 10}, // Braços
		{5, 11}, {6, 12}, {11, 12},              // Torso
		{11, 13}, {13, 15}, {12, 14}, {14, 16},  // Pernas
		{0, 1}, {0, 2}, {1, 3}, {2, 4},          // Cabeça
	}

	for _, conn := range connections {
		kp1 := pose.Keypoints[conn[0]]
		kp2 := pose.Keypoints[conn[1]]

		// Desenha linha apenas se ambos keypoints são confiáveis
		if kp1.Confidence > 0.3 && kp2.Confidence > 0.3 {
			gocv.Line(img,
				image.Pt(int(kp1.X), int(kp1.Y)),
				image.Pt(int(kp2.X), int(kp2.Y)),
				skeletonColor, 2)
		}
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