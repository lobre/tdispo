BINARY_NAME=tdispo

.PHONY: all tailwindcss install build run

all: run

tailwindcss:
	tailwindcss --input ./tailwind.css --output ./static/style.css --minify

install:
	go install

build: tailwindcss
	go build -o ${BINARY_NAME}

run: build
	./${BINARY_NAME}

