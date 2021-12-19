BINARY_NAME=tdispo

.PHONY: all tailwindcss build install run watch

all: run

tailwindcss:
	tailwindcss --input ./tailwind.css --output ./static/style.css --minify

build: tailwindcss
	go build -o ${BINARY_NAME}

install: tailwindcss
	go install

run: build
	./${BINARY_NAME}

watch:
	@inotifywait -m -qr -e close_write . | grep -E "\.(go|html)$$" --line-buffered | \
	while read path events file; do \
		if [ -n "$$pid" ]; then kill "$$pid"; fi; \
		make --no-print-directory & \
		pid=$$!; \
	done

