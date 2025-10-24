package shoplifting

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"gocv.io/x/gocv"
	"poc-camera/config"
)

// TrackedPerson representa uma pessoa sendo rastreada ao longo do tempo
type TrackedPerson struct {
	ID              int
	LastSeen        time.Time
	Positions       []image.Point
	LoiteringTime   time.Duration
	SuspiciousCount int
	FirstSeen       time.Time
	LastSuspiciousMovement time.Time // Para cooldown
	LastLogTimes    map[string]time.Time // Para throttling de logs por tipo
}

// SuspiciousBehavior representa um comportamento suspeito detectado
type SuspiciousBehavior struct {
	Type        string
	Confidence  float32
	Description string
	Details     string // Detalhes específicos sobre o que foi detectado
	PersonID    int
	Location    image.Point
	ShouldLog   bool   // Se deve mostrar no log (throttling de 1 vez por segundo)
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
	trackedPeople  map[int]*TrackedPerson
	nextPersonID   int
	config         *config.Config
	valuableItems  map[int]string
	frameCount     int
}

// NewShopliftingDetector cria um novo detector de shoplifting
func NewShopliftingDetector(objectDetector ObjectDetector, cfg *config.Config) (*ShopliftingDetector, error) {
	fmt.Println("✅ Sistema funcionando com:")
	fmt.Println("   • Detecção de objetos (365 classes)")
	fmt.Println("   • Tracking de pessoas")
	fmt.Println("   • Detecção de loitering (tempo)")
	fmt.Println("   • Proximidade com itens valiosos")
	fmt.Println("   • Análise comportamental baseada em movimento")

	return &ShopliftingDetector{
		objectDetector: objectDetector,
		trackedPeople:  make(map[int]*TrackedPerson),
		nextPersonID:   1,
		config:         cfg,
		valuableItems:  config.GetValuableItems(),
	}, nil
}

// Close libera recursos do detector
func (sd *ShopliftingDetector) Close() {
	// Nenhum recurso adicional para liberar
}

// DetectShoplifting executa detecção completa de shoplifting
func (sd *ShopliftingDetector) DetectShoplifting(img gocv.Mat) ([]DetectionResult, []SuspiciousBehavior) {
	// 1. Detecta objetos (incluindo pessoas)
	detections := sd.objectDetector.Detect(img)

	// 2. Filtra pessoas e objetos valiosos
	people := sd.filterPeople(detections)
	valuableObjects := sd.filterValuableObjects(detections)

	// 3. Atualiza tracking de pessoas
	sd.updateTracking(people)

	// 4. Analisa comportamentos suspeitos
	suspiciousBehaviors := sd.analyzeBehaviors(people, valuableObjects)

	// 5. Remove pessoas que não são mais vistas
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



// updateTracking atualiza tracking de pessoas
func (sd *ShopliftingDetector) updateTracking(people []DetectionResult) {
	currentTime := time.Now()

	// Associa detecções com pessoas rastreadas
	for _, person := range people {
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
				ID:           trackedID,
				FirstSeen:    currentTime,
				LastSeen:     currentTime,
				Positions:    []image.Point{personCenter},
				LastLogTimes: make(map[string]time.Time),
			}
		}

		// Atualiza pessoa rastreada
		tracked := sd.trackedPeople[trackedID]
		tracked.LastSeen = currentTime
		tracked.Positions = append(tracked.Positions, personCenter)

		// Calcula tempo de permanência
		tracked.LoiteringTime = currentTime.Sub(tracked.FirstSeen)

		// Limita histórico de posições
		maxPositions := 30 // aproximadamente 1 segundo a 30fps
		if len(tracked.Positions) > maxPositions {
			tracked.Positions = tracked.Positions[1:]
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

// shouldLogBehavior verifica se um comportamento deve ser logado baseado em throttling (1x por segundo)
func (sd *ShopliftingDetector) shouldLogBehavior(tracked *TrackedPerson, behaviorType string) bool {
	currentTime := time.Now()

	if lastLog, exists := tracked.LastLogTimes[behaviorType]; exists {
		// Se logou há menos de 1 segundo, não loga novamente
		if currentTime.Sub(lastLog).Seconds() < 1.0 {
			return false
		}
	}

	// Atualiza timestamp do último log para este tipo
	tracked.LastLogTimes[behaviorType] = currentTime
	return true
}

// analyzeBehaviors analisa comportamentos suspeitos
func (sd *ShopliftingDetector) analyzeBehaviors(people []DetectionResult, valuableObjects []DetectionResult) []SuspiciousBehavior {
	var behaviors []SuspiciousBehavior

	for id, tracked := range sd.trackedPeople {
		// Análise de tempo de permanência (loitering)
		if tracked.LoiteringTime.Seconds() > sd.config.LoiteringTimeThreshold {
			behaviors = append(behaviors, SuspiciousBehavior{
				Type:        "PERMANENCIA_EXCESSIVA",
				Confidence:  float32(math.Min(tracked.LoiteringTime.Seconds()/30.0, 1.0)),
				Description: fmt.Sprintf("Pessoa permanecendo na área por %.1f segundos", tracked.LoiteringTime.Seconds()),
				Details:     fmt.Sprintf("Limite: %.1fs | Tempo atual: %.1fs", sd.config.LoiteringTimeThreshold, tracked.LoiteringTime.Seconds()),
				PersonID:    id,
				Location:    tracked.Positions[len(tracked.Positions)-1],
				ShouldLog:   sd.shouldLogBehavior(tracked, "PERMANENCIA_EXCESSIVA"),
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
					behaviorKey := fmt.Sprintf("PROXIMIDADE_SUSPEITA_%s", valuable.Label)
					behaviors = append(behaviors, SuspiciousBehavior{
						Type:        "PROXIMIDADE_SUSPEITA",
						Confidence:  float32(1.0 - distance/sd.config.ProximityThreshold),
						Description: fmt.Sprintf("Próximo a %s", valuable.Label),
						Details:     fmt.Sprintf("Distância: %.1f pixels | Limite: %.1f pixels", distance, sd.config.ProximityThreshold),
						PersonID:    id,
						Location:    lastPos,
						ShouldLog:   sd.shouldLogBehavior(tracked, behaviorKey),
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

				movementAnalysis := sd.analyzeSuspiciousMovement(recentPositions)

				// Threshold mais alto para evitar false positives
				if movementAnalysis.Score > 0.9 { // Era 0.8, agora 0.9
					// Formata os detalhes em uma string limpa
					detailsStr := ""
					if len(movementAnalysis.Details) > 0 {
						detailsStr = strings.Join(movementAnalysis.Details, " | ")
					}

					behaviors = append(behaviors, SuspiciousBehavior{
						Type:        "MOVIMENTO_SUSPEITO",
						Confidence:  movementAnalysis.Score,
						Description: "Padrão de movimento altamente suspeito detectado",
						Details:     detailsStr,
						PersonID:    id,
						Location:    tracked.Positions[len(tracked.Positions)-1],
						ShouldLog:   sd.shouldLogBehavior(tracked, "MOVIMENTO_SUSPEITO"),
					})
					// Atualiza timestamp do último alerta
					tracked.LastSuspiciousMovement = currentTime
				}
			}
		}

	}

	return behaviors
}


// MovementAnalysis contém resultado da análise de movimento
type MovementAnalysis struct {
	Score   float32
	Details []string
}

// analyzeSuspiciousMovement analisa padrões de movimento suspeitos (otimizado)
func (sd *ShopliftingDetector) analyzeSuspiciousMovement(positions []image.Point) MovementAnalysis {
	if len(positions) < 10 {
		return MovementAnalysis{Score: 0, Details: []string{}}
	}

	suspiciousScore := float32(0)
	var details []string

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
			details = append(details, fmt.Sprintf("Movimento errático: %.1f%% mudanças de direção", changeRate*100))
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
		details = append(details, fmt.Sprintf("Movimento circular em área pequena: raio %.1f pixels", maxDistance))
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
			details = append(details, fmt.Sprintf("Velocidade inconsistente: variação %.1f (média %.1f px/frame)", variance, avgSpeed))
		}
	}

	// Limita o score entre 0 e 1
	if suspiciousScore > 1.0 {
		suspiciousScore = 1.0
	}

	return MovementAnalysis{
		Score:   suspiciousScore,
		Details: details,
	}
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