# POC Camera - YOLOv11 Object365 Detection

Este projeto é uma Prova de Conceito (PoC) que utiliza **Go**, **GoCV** (OpenCV para Go) e o modelo **YOLOv11 Object365** para detectar objetos em tempo real através de uma webcam.

O sistema é capaz de detectar **365 categorias diferentes de objetos** usando o dataset Object365, oferecendo detecção muito mais abrangente que modelos tradicionais.

## 🎯 Características

- **YOLOv11**: Última versão do YOLO com melhor precisão e velocidade
- **Object365**: Dataset com 365 classes de objetos (vs 80 do COCO tradicional)
- **Detecção em tempo real**: Processamento via webcam
- **Otimizado**: Modelo ONNX para inferência rápida
- **Multilíngue**: Classes em português e inglês

## 📋 Pré-requisitos

Antes de começar, você precisará ter os seguintes softwares instalados:

- **[Go](https://golang.org/dl/)** (versão 1.18 ou superior)
- **[OpenCV](https://opencv.org/)** (versão 4.x)
- **Python 3** (para download e conversão de modelos)
- **pkg-config**

### 🍎 Instalação no macOS

```bash
# Instalar dependências via Homebrew
brew install opencv pkg-config python3

# Instalar ultralytics para conversão de modelos
pip3 install ultralytics
```

### 🐧 Instalação no Linux (Ubuntu/Debian)

```bash
# Instalar dependências
sudo apt-get update
sudo apt-get install libopencv-dev pkg-config python3 python3-pip

# Instalar ultralytics
pip3 install ultralytics
```

## 🚀 Instalação e Uso

### 1. Clone o repositório

```bash
git clone <repository-url>
cd poc-camera
```

### 2. Download automático dos modelos

Execute o script de download que baixa automaticamente o modelo YOLOv11 Object365:

```bash
./download_models.sh
```

Este script irá:
- ✅ Baixar modelo YOLOv11 Object365 (365 classes) do Hugging Face
- ✅ Converter automaticamente para formato ONNX
- ✅ Criar arquivos de classes em português e inglês
- ✅ Configurar tudo automaticamente

### 3. Executar a aplicação

```bash
make run
```

Ou diretamente:

```bash
go run main.go
```

## 🎮 Controles

- **ESC** ou **Q**: Sair da aplicação
- A detecção acontece automaticamente em tempo real

### ⚠️ Permissões no macOS

Na primeira execução, o macOS solicitará permissão para acessar a câmera. Se não aparecer automaticamente:

`Configurações do Sistema > Privacidade e Segurança > Câmera`

Habilite o acesso para seu terminal (Terminal, iTerm2, etc.).

## 📊 O que o Sistema Detecta

O modelo Object365 pode identificar **365 categorias** diferentes de objetos, incluindo:

### 👥 Pessoas e Vestuário
- Pessoas, tênis, chapéu, óculos, bolsa, etc.

### 🚗 Veículos
- Carros, ônibus, motocicletas, aviões, barcos, etc.

### 🏠 Casa e Móveis
- Cadeiras, mesas, sofás, camas, TVs, etc.

### 🍎 Comida e Bebida
- Frutas, pizza, sanduíches, bebidas, etc.

### 🐕 Animais
- Cachorros, gatos, cavalos, pássaros, etc.

### 📱 Eletrônicos
- Celulares, laptops, câmeras, etc.

### ⚽ Esportes e Lazer
- Bolas, raquetes, skates, etc.

*E muito mais! Veja os arquivos `models/object365.names` ou `models/object365_real.names` para a lista completa.*

## 🛠️ Estrutura do Projeto

```
poc-camera/
├── main.go                 # Código principal da aplicação
├── download_models.sh      # Script de download automático
├── Makefile               # Comandos de build e execução
├── models/                # Modelos e arquivos de classes (criado automaticamente)
│   ├── yolo11n_object365.pt    # Modelo PyTorch
│   ├── yolo11n_object365.onnx  # Modelo ONNX otimizado
│   ├── object365.names         # Classes em português
│   └── object365_real.names    # Classes em inglês
└── README.md              # Este arquivo
```

## 🔧 Comandos Makefile

```bash
make run    # Executar aplicação
make build  # Compilar binário
make clean  # Limpar arquivos de build
make help   # Mostrar ajuda
```

## 🏆 Vantagens do Object365

| Aspecto | COCO (tradicional) | Object365 (este projeto) |
|---------|-------------------|---------------------------|
| **Classes** | 80 | **365** |
| **Variedade** | Básica | **Muito abrangente** |
| **Precisão** | Boa | **Excelente** |
| **Casos de uso** | Limitado | **Amplo** |

## 📝 Próximos Passos

- 🎯 Adicionar detecção de mãos/gestos
- 🎨 Melhorar interface visual
- 📊 Adicionar métricas de performance
- 🔄 Suporte a múltiplas câmeras
- 💾 Gravação de detecções
