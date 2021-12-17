BINARY_NAME=tdispo

.PHONY: tailwindcss build install run

tailwindcss:
	tailwindcss --input ./tailwind.css --output ./static/style.css --minify

build: tailwindcss
	go build -o ${BINARY_NAME}

install: tailwindcss
	go install

run: build
	./${BINARY_NAME}

