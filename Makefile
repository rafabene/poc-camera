# Makefile para POC Camera - Object Detection

.PHONY: all build clean run help

# Configurações
BINARY_NAME=poc-camera
MAIN_FILES=*.go

# Detecta o sistema operacional
UNAME_S := $(shell uname -s)

# Flags específicas para macOS
ifeq ($(UNAME_S),Darwin)
    BUILD_FLAGS=-ldflags="-s -w" -tags=opencv4
    CGO_LDFLAGS=-framework CoreFoundation -framework AVFoundation -framework QuartzCore
else
    BUILD_FLAGS=-ldflags="-s -w"
    CGO_LDFLAGS=
endif

# Comandos principais
all: build

build:
	@echo "🔨 Compilando $(BINARY_NAME)..."
	CGO_LDFLAGS="$(CGO_LDFLAGS)" go build $(BUILD_FLAGS) -o $(BINARY_NAME) $(MAIN_FILES)
	@echo "✅ Build concluído: $(BINARY_NAME)"

run: build
	@echo "🚀 Executando detecção de objetos..."
	./$(BINARY_NAME)

clean:
	@echo "🧹 Limpando arquivos..."
	rm -f $(BINARY_NAME)
	rm -f *.log
	@echo "✅ Limpeza concluída"


install-deps:
	@echo "📦 Instalando dependências..."
	go mod tidy
	@echo "✅ Dependências instaladas"

test:
	@echo "📹 Testando aplicação..."
	go run config/*.go *.go

help:
	@echo "🔍 POC Camera - Object Detection"
	@echo "===============================\n"
	@echo "Comandos disponíveis:"
	@echo "  make build        - Compila o projeto"
	@echo "  make run          - Executa detecção de objetos"
	@echo "  make clean        - Remove arquivos gerados"
	@echo "  make install-deps - Instala dependências"
	@echo "  make test         - Testa a aplicação"
	@echo "  make help         - Mostra esta ajuda\n"
	@echo "Exemplo:"
	@echo "  make run          # Inicia detector de objetos"

# Target para desenvolvimento
dev-build:
	@echo "🔧 Build de desenvolvimento..."
	go build -race $(BUILD_FLAGS) -o $(BINARY_NAME)-dev $(MAIN_FILES)

