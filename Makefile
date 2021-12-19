BINARY_NAME=tdispo
LOCALE="en_US"

.PHONY: all tailwindcss build install run watch

all: run

tailwindcss:
	tailwindcss --input ./tailwind.css --output ./static/tailwind.css --minify

build: tailwindcss
	go build -o ${BINARY_NAME}

install: tailwindcss
	go install

run: build
	./${BINARY_NAME} -locale $(LOCALE)

watch:
	@inotifywait -m -qr -e close_write . | grep -E "\.(go|html)$$" --line-buffered | \
	while read path events file; do \
		if [ -n "$$pid" ]; then kill "$$pid"; fi; \
		make --no-print-directory & \
		pid=$$!; \
	done

