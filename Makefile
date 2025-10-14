# Makefile para o projeto de POC com GoCV

.PHONY: run build clean help

# Comando padrão: executa a aplicação
default: run

# run: Executa o programa principal
run:
	@echo "Executando a aplicação..."
	go run main.go

# build: Compila o programa e gera um executável
build:
	@echo "Compilando o binário..."
	go build -o poc-camera main.go

# clean: Remove o executável compilado
clean:
	@echo "Limpando arquivos de build..."
	rm -f poc-camera

# help: Mostra esta mensagem de ajuda
help:
	@echo "Comandos disponíveis:"
	@echo "  make run    - Executa a aplicação em tempo real."
	@echo "  make build  - Compila e cria o executável 'poc-camera'."
	@echo "  make clean  - Remove o executável gerado."
	@echo "  make help   - Mostra esta ajuda."

