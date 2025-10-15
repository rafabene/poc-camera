#!/bin/bash

# Script para download dos modelos YOLOv11 Object365
# Autor: Claude Code
# Data: $(date +%Y-%m-%d)

set -e  # Para o script se houver erro

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Função para logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Função para verificar se comando existe
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "Comando '$1' não encontrado. Por favor, instale-o primeiro."
        exit 1
    fi
}

# Função para download com progresso
download_file() {
    local url=$1
    local output=$2
    local description=$3

    log_info "Baixando $description..."

    local success=false

    if command -v wget &> /dev/null; then
        if wget --progress=bar:force:noscroll -O "$output" "$url" 2>/dev/null; then
            success=true
        fi
    elif command -v curl &> /dev/null; then
        if curl -L --progress-bar -o "$output" "$url" 2>/dev/null; then
            success=true
        fi
    else
        log_error "wget ou curl não encontrado. Instale um dos dois para continuar."
        exit 1
    fi

    if [ "$success" = true ]; then
        log_success "$description baixado com sucesso!"
        return 0
    else
        log_error "Falha ao baixar $description"
        return 1
    fi
}

# Função para verificar integridade do arquivo
verify_file() {
    local file=$1
    local min_size=$2
    local description=$3

    if [ ! -f "$file" ]; then
        log_error "$description não encontrado: $file"
        return 1
    fi

    local file_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)

    if [ "$file_size" -lt "$min_size" ]; then
        log_error "$description parece estar corrompido (tamanho: $file_size bytes, mínimo: $min_size bytes)"
        return 1
    fi

    log_success "$description verificado ($(($file_size / 1024 / 1024)) MB)"
    return 0
}

# Banner
echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                  YOLOv11 Object365 Downloader               ║"
echo "║                                                              ║"
echo "║  Este script baixa os modelos necessários para o projeto    ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Verificar se estamos no diretório correto
if [ ! -f "main.go" ]; then
    log_error "Execute este script no diretório raiz do projeto (onde está o main.go)"
    exit 1
fi

# Criar diretório models se não existir
if [ ! -d "models" ]; then
    log_info "Criando diretório models..."
    mkdir -p models
fi

cd models

# URL do modelo YOLOv11 Object365 real (365 classes)
YOLO11N_OBJECT365_URL="https://huggingface.co/NRtred/yolo11n_object365/resolve/main/yolo11n_object365.pt"

log_info "Iniciando download dos modelos..."

# 1. Download do modelo YOLOv11 Object365 (365 classes)
if [ ! -f "yolo11n_object365.onnx" ]; then
    log_warning "Modelo ONNX não encontrado. Baixando modelo Object365 PyTorch..."

    if [ ! -f "yolo11n_object365.pt" ]; then
        log_info "Baixando YOLOv11 Object365 do Hugging Face..."

        if download_file "$YOLO11N_OBJECT365_URL" "yolo11n_object365.pt" "YOLOv11 Object365 modelo PyTorch"; then
            if verify_file "yolo11n_object365.pt" 5000000 "YOLOv11 Object365 PyTorch"; then
                log_success "Modelo Object365 baixado com sucesso!"
            else
                log_error "Arquivo Object365 corrompido"
                exit 1
            fi
        else
            log_error "Falha ao baixar modelo Object365"
            log_info "Verifique sua conexão e tente novamente"
            exit 1
        fi
    fi

    log_info "Convertendo modelo Object365 para ONNX..."
    if command -v python3 &> /dev/null; then
        python3 -c "
try:
    from ultralytics import YOLO
    print('🔄 Convertendo YOLOv11 Object365 para ONNX...')
    model = YOLO('yolo11n_object365.pt')
    print(f'📊 Modelo com {model.model.model[-1].nc} classes')
    model.export(format='onnx')
    print('✅ Conversão para ONNX concluída!')
except Exception as e:
    print(f'❌ Erro na conversão: {e}')
    exit(1)
"
        if [ $? -eq 0 ] && [ -f "yolo11n_object365.onnx" ]; then
            log_success "Modelo ONNX Object365 criado com sucesso!"
        else
            log_error "Falha na conversão para ONNX"
            exit 1
        fi
    else
        log_error "Python3 necessário para conversão ONNX"
        exit 1
    fi
else
    log_success "Modelo ONNX Object365 já existe!"
fi

# 2. Criar arquivo de classes Object365 em português
if [ ! -f "object365.names" ]; then
    log_info "Criando arquivo de classes Object365 em português..."

    cat > object365.names << 'EOF'
pessoa
tênis
cadeira
outros sapatos
chapéu
carro
lâmpada
óculos
garrafa
mesa
xícara
postes de luz
armário/estante
bolsa/maleta
pulseira
prato
quadro/moldura
capacete
livro
luvas
caixa de armazenamento
barco
sapatos de couro
flor
banco
planta em vaso
tigela/bacia
bandeira
travesseiro
botas
vaso
microfone
colar
anel
SUV
taça de vinho
cinto
monitor/TV
mochila
guarda-chuva
semáforo
alto-falante
relógio
gravata
lixeira
chinelos
bicicleta
banqueta
barril/balde
van
sofá
sandálias
cesta
tambor
caneta/lápis
ônibus
pássaro selvagem
salto alto
motocicleta
violão
tapete
celular
pão
câmera
enlatados
caminhão
cone de trânsito
prato (musical)
salva-vidas
toalha
brinquedo de pelúcia
vela
veleiro
laptop
toldo
cama
torneira
tenda
cavalo
espelho
tomada elétrica
pia
maçã
ar condicionado
faca
taco de hockey
remo
caminhonete
garfo
placa de trânsito
balão
tripé
cachorro
colher
relógio
panela
vaca
bolo
mesa de jantar
ovelha
cabide
lousa/quadro branco
guardanapo
outros peixes
laranja/tangerina
artigos de higiene
teclado
tomate
lanterna
veículo de máquinas
ventilador
verduras verdes
banana
luva de baseball
avião
mouse
trem
abóbora
futebol
esqui
bagagem
criado-mudo
bule de chá
telefone
carrinho
fone de ouvido
carro esportivo
placa de pare
sobremesa
scooter
carrinho de bebê
guindaste
controle remoto
geladeira
forno
limão
pato
taco de baseball
câmera de vigilância
gato
jarro
brócolis
piano
pizza
elefante
skate
prancha de surf
arma
sapatos de patinação e esqui
fogão a gás
rosquinha
gravata borboleta
cenoura
vaso sanitário
pipa
morango
outras bolas
pá
pimenta
gabinete do computador
papel higiênico
produtos de limpeza
pauzinhos
micro-ondas
pombo
baseball
tábua de corte
mesa de centro
mesa lateral
tesoura
marcador
torta
escada
snowboard
biscoitos
radiador
hidrante
basquete
zebra
uva
girafa
batata
salsicha
triciclo
violino
ovo
extintor
doce
caminhão de bombeiros
bilhar
conversor
banheira
cadeira de rodas
taco de golfe
maleta
pepino
charuto/cigarro
pincel
pêra
caminhão pesado
hambúrguer
exaustor
cabo de extensão
pegador
raquete de tênis
pasta
futebol americano
fone de ouvido
máscara
chaleira
tênis
navio
balanço
máquina de café
escorregador
carruagem
cebola
vagem
projetor
frisbee
máquina de lavar/secar
frango
impressora
melancia
saxofone
lenço
escova de dentes
sorvete
balão de ar quente
violoncelo
batata frita
balança
troféu
repolho
cachorro-quente
liquidificador
pêssego
arroz
carteira/bolsa
vôlei
veado
ganso
fita
tablet
cosméticos
trompete
abacaxi
bola de golfe
ambulância
parquímetro
manga
chave
obstáculo
vara de pescar
medalha
flauta
escova
pinguim
megafone
milho
alface
alho
cisne
helicóptero
cebolinha
sanduíche
nozes
placa de limite de velocidade
fogão de indução
vassoura
trombone
ameixa
riquixá
peixe dourado
kiwi
roteador/modem
cartas de poker
torradeira
camarão
sushi
queijo
papel de nota
cereja
alicate
CD
macarrão
martelo
taco de sinuca
abacate
melão hami
frasco
cogumelo
chave de fenda
sabão
gravador
urso
berinjela
apagador de quadro
coco
fita métrica/régua
porco
chuveiro
globo
batatas fritas
bife
placa de faixa de pedestres
grampeador
camelo
fórmula 1
romã
lava-louças
caranguejo
hoverboard
almôndega
panela de arroz
tuba
calculadora
mamão
antílope
papagaio
foca
borboleta
haltere
burro
leão
mictório
golfinho
furadeira elétrica
secador de cabelo
pastel de ovo
água-viva
esteira
isqueiro
toranja
tabuleiro de jogo
esfregão
rabanete
baozi
alvo
francês
rolinhos primavera
macaco
coelho
estojo de lápis
iaque
repolho roxo
binóculos
aspargo
barra
vieira
macarrão
pente
dumpling
ostra
raquete de tênis de mesa
pincel de cosméticos/lápis de olho
motosserra
borracha
lagosta
durian
quiabo
batom
espelho de cosméticos
curling
tênis de mesa
EOF

    log_success "Arquivo de classes em português criado!"
else
    log_success "Arquivo de classes em português já existe!"
fi

# 3. Criar arquivo de classes Object365 em inglês
if [ ! -f "object365_real.names" ]; then
    log_info "Criando arquivo de classes Object365 em inglês..."

    cat > object365_real.names << 'EOF'
Person
Sneakers
Chair
Other Shoes
Hat
Car
Lamp
Glasses
Bottle
Desk
Cup
Street Lights
Cabinet/shelf
Handbag/Satchel
Bracelet
Plate
Picture/Frame
Helmet
Book
Gloves
Storage box
Boat
Leather Shoes
Flower
Bench
Potted Plant
Bowl/Basin
Flag
Pillow
Boots
Vase
Microphone
Necklace
Ring
SUV
Wine Glass
Belt
Monitor/TV
Backpack
Umbrella
Traffic Light
Speaker
Watch
Tie
Trash bin Can
Slippers
Bicycle
Stool
Barrel/bucket
Van
Couch
Sandals
Basket
Drum
Pen/Pencil
Bus
Wild Bird
High Heels
Motorcycle
Guitar
Carpet
Cell Phone
Bread
Camera
Canned
Truck
Traffic cone
Cymbal
Lifesaver
Towel
Stuffed Toy
Candle
Sailboat
Laptop
Awning
Bed
Faucet
Tent
Horse
Mirror
Power outlet
Sink
Apple
Air Conditioner
Knife
Hockey Stick
Paddle
Pickup Truck
Fork
Traffic Sign
Balloon
Tripod
Dog
Spoon
Clock
Pot
Cow
Cake
Dining Table
Sheep
Hanger
Blackboard/Whiteboard
Napkin
Other Fish
Orange/Tangerine
Toiletry
Keyboard
Tomato
Lantern
Machinery Vehicle
Fan
Green Vegetables
Banana
Baseball Glove
Airplane
Mouse
Train
Pumpkin
Soccer
Skiboard
Luggage
Nightstand
Tea pot
Telephone
Trolley
Head Phone
Sports Car
Stop Sign
Dessert
Scooter
Stroller
Crane
Remote
Refrigerator
Oven
Lemon
Duck
Baseball Bat
Surveillance Camera
Cat
Jug
Broccoli
Piano
Pizza
Elephant
Skateboard
Surfboard
Gun
Skating and Skiing shoes
Gas stove
Donut
Bow Tie
Carrot
Toilet
Kite
Strawberry
Other Balls
Shovel
Pepper
Computer Box
Toilet Paper
Cleaning Products
Chopsticks
Microwave
Pigeon
Baseball
Cutting/chopping Board
Coffee Table
Side Table
Scissors
Marker
Pie
Ladder
Snowboard
Cookies
Radiator
Fire Hydrant
Basketball
Zebra
Grape
Giraffe
Potato
Sausage
Tricycle
Violin
Egg
Fire Extinguisher
Candy
Fire Truck
Billiards
Converter
Bathtub
Wheelchair
Golf Club
Briefcase
Cucumber
Cigar/Cigarette
Paint Brush
Pear
Heavy Truck
Hamburger
Extractor
Extension Cord
Tong
Tennis Racket
Folder
American Football
earphone
Mask
Kettle
Tennis
Ship
Swing
Coffee Machine
Slide
Carriage
Onion
Green beans
Projector
Frisbee
Washing Machine/Drying Machine
Chicken
Printer
Watermelon
Saxophone
Tissue
Toothbrush
Ice cream
Hot-air balloon
Cello
French Fries
Scale
Trophy
Cabbage
Hot dog
Blender
Peach
Rice
Wallet/Purse
Volleyball
Deer
Goose
Tape
Tablet
Cosmetics
Trumpet
Pineapple
Golf Ball
Ambulance
Parking meter
Mango
Key
Hurdle
Fishing Rod
Medal
Flute
Brush
Penguin
Megaphone
Corn
Lettuce
Garlic
Swan
Helicopter
Green Onion
Sandwich
Nuts
Speed Limit Sign
Induction Cooker
Broom
Trombone
Plum
Rickshaw
Goldfish
Kiwi fruit
Router/modem
Poker Card
Toaster
Shrimp
Sushi
Cheese
Notepaper
Cherry
Pliers
CD
Pasta
Hammer
Cue
Avocado
Hami melon
Flask
Mushroom
Screwdriver
Soap
Recorder
Bear
Eggplant
Board Eraser
Coconut
Tape Measure/Ruler
Pig
Showerhead
Globe
Chips
Steak
Crosswalk Sign
Stapler
Camel
Formula 1
Pomegranate
Dishwasher
Crab
Hoverboard
Meatball
Rice Cooker
Tuba
Calculator
Papaya
Antelope
Parrot
Seal
Butterfly
Dumbbell
Donkey
Lion
Urinal
Dolphin
Electric Drill
Hair Dryer
Egg tart
Jellyfish
Treadmill
Lighter
Grapefruit
Game board
Mop
Radish
Baozi
Target
French
Spring Rolls
Monkey
Rabbit
Pencil Case
Yak
Red Cabbage
Binoculars
Asparagus
Barbell
Scallop
Noddles
Comb
Dumpling
Oyster
Table Tennis paddle
Cosmetics Brush/Eyeliner Pencil
Chainsaw
Eraser
Lobster
Durian
Okra
Lipstick
Cosmetics Mirror
Curling
Table Tennis
EOF

    log_success "Arquivo de classes em inglês criado!"
else
    log_success "Arquivo de classes em inglês já existe!"
fi

cd ..

# Resumo final
echo ""
log_info "═══════════════════════════════════════════════════════════════"
log_info "                        RESUMO DO DOWNLOAD"
log_info "═══════════════════════════════════════════════════════════════"

if [ -f "models/yolo11n_object365.pt" ]; then
    log_success "✓ Modelo YOLOv11 Object365 PyTorch baixado"
else
    log_warning "✗ Modelo YOLOv11 Object365 PyTorch não encontrado"
fi

if [ -f "models/yolo11n_object365.onnx" ]; then
    log_success "✓ Modelo ONNX Object365 (365 classes) disponível"
else
    log_warning "✗ Modelo ONNX Object365 não encontrado"
fi

if [ -f "models/object365.names" ]; then
    log_success "✓ Classes em português criadas"
fi

if [ -f "models/object365_real.names" ]; then
    log_success "✓ Classes em inglês criadas"
fi

echo ""
log_info "Para usar o projeto:"
echo -e "${GREEN}  make run${NC}"
echo ""
log_info "O projeto agora está configurado com:"
echo -e "${GREEN}  ✓ YOLOv11 Object365 (365 classes)${NC}"
echo -e "${GREEN}  ✓ Modelo ONNX otimizado${NC}"
echo -e "${GREEN}  ✓ Classes em português e inglês${NC}"

echo ""
log_success "Download Object365 concluído! 🎉"