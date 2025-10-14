# Prova de Conceito: Detecção de Objetos na Mão com Go e OpenCV

Este projeto é uma Prova de Conceito (PoC) que utiliza a linguagem Go, a biblioteca GoCV (bindings de Go para OpenCV) e o modelo de IA YOLOv3-tiny para detectar objetos em tempo real através de uma webcam.

O objetivo final é identificar qual objeto está sendo segurado por uma pessoa.

## Pré-requisitos

Antes de começar, você precisará ter os seguintes softwares instalados:

-   [Go](https://golang.org/dl/) (versão 1.18 ou superior)
-   [OpenCV](https://opencv.org/) (versão 4.x)
-   `pkg-config`

### Instalação no macOS

A maneira mais fácil de instalar as dependências no macOS é usando o [Homebrew](https://brew.sh/):

```sh
brew install opencv pkg-config
```

## Instalação do Projeto

1.  **Clone o repositório (ou use os arquivos existentes).**

2.  **Baixe as dependências do Go:**
    O Go irá baixar as dependências automaticamente ao executar o projeto.

3.  **Baixe os arquivos do modelo YOLO:**
    Os arquivos do modelo são necessários para a detecção de objetos. Se o diretório `models` ainda não existir com os arquivos, crie-o e baixe os arquivos:
    ```sh
    mkdir models
    curl -L -o models/yolov3-tiny.weights https://pjreddie.com/media/files/yolov3-tiny.weights
    curl -L -o models/yolov3-tiny.cfg https://raw.githubusercontent.com/pjreddie/darknet/master/cfg/yolov3-tiny.cfg
    curl -L -o models/coco.names https://raw.githubusercontent.com/pjreddie/darknet/master/data/coco.names
    ```

## Como Executar

Com o `Makefile` fornecido, você pode simplesmente usar o comando:

```sh
make run
```

Alternativamente, você pode usar o comando padrão do Go:

```sh
go run main.go
```

### ⚠️ Permissão da Câmera no macOS

Na primeira vez que você executar o programa, o macOS pode solicitar permissão para que seu terminal acesse a câmera. Se isso não acontecer automaticamente, você precisará ir em:

`Preferências do Sistema > Privacidade e Segurança > Câmera`

E habilitar o acesso para o seu aplicativo de terminal (ex: Terminal, iTerm2).

## Próximos Passos

O código atual detecta objetos genéricos. O próximo passo é adicionar um segundo modelo para detectar a **mão** e então criar a lógica que verifica quando a caixa de detecção de um objeto se sobrepõe à caixa de detecção da mão.
