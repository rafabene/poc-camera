package config

// Config centraliza todas as configurações do sistema
type Config struct {
	// Modelos
	ObjectDetectionModel string
	ClassNamesFile       string

	// Thresholds de detecção
	ConfidenceThreshold float32
	NMSThreshold        float32
	MinObjectSize       int

	// Configurações de shoplifting
	HidingBehaviorThreshold    float32
	LoiteringTimeThreshold     float64
	ProximityThreshold         float64

	// Interface
	WindowName        string
	InputSize         int
	NumDetections     int
	NumAttributes     int
	MaxValidClassID   int

	// Performance
	MaxTrackedPeople   int
	TrackerTimeout     float64
}

// DefaultConfig retorna configuração padrão
func DefaultConfig() *Config {
	return &Config{
		// Modelos
		ObjectDetectionModel: "models/yolo11n_object365.onnx",
		ClassNamesFile:       "models/object365.names",

		// Thresholds de detecção
		ConfidenceThreshold: 0.25,
		NMSThreshold:        0.4,
		MinObjectSize:       20,

		// Configurações de shoplifting
		HidingBehaviorThreshold:    0.7,
		LoiteringTimeThreshold:     20.0, // segundos
		ProximityThreshold:         80.0, // pixels

		// Interface
		WindowName:      "🛡️ Shoplifting Detector - YOLO v11 Object Detection",
		InputSize:       640,
		NumDetections:   8400,
		NumAttributes:   369, // 4 coordenadas + 365 classes Object365
		MaxValidClassID: 364, // Object365: 365 classes (0-364)

		// Performance
		MaxTrackedPeople: 50,
		TrackerTimeout:   5.0, // segundos
	}
}

// GetValuableItems define IDs de classes consideradas valiosas
func GetValuableItems() map[int]string {
	return map[int]string{
		// Eletrônicos (ajustar IDs conforme arquivo de classes)
		50:  "telefone",
		51:  "notebook",
		52:  "tablet",
		53:  "câmera",
		54:  "fone de ouvido",

		// Acessórios
		100: "bolsa",
		101: "carteira",
		102: "relógio",

		// Roupas de valor
		200: "casaco",
		201: "tênis",
		202: "jaqueta",

		// Cosméticos/Perfumes
		300: "perfume",
		301: "maquiagem",

		// Bebidas/Comidas premium
		350: "vinho",
		351: "whisky",
		352: "chocolate premium",
	}
}