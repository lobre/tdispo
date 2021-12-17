BINARY_NAME=tdispo

.PHONY: all tailwindcss build run install

all: tailwindcss build run

tailwindcss:
	tailwindcss --input ./tailwind.css --output ./static/style.css --minify

build:
	go build -o ${BINARY_NAME}

run:
	./${BINARY_NAME}

install:
	go install

