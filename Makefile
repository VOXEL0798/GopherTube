.PHONY: install

# Install GopherTube (build, install binary, install man page)
install:
	go mod tidy
	go build -o gophertube main.go
	sudo cp gophertube /usr/local/bin/
	sudo mkdir -p /usr/local/share/man/man1
	sudo cp man/gophertube.1 /usr/local/share/man/man1/
	sudo mandb
	@echo "GopherTube installed successfully!"
	@echo "Run 'gophertube --help' to get started." 