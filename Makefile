.PHONY: build build-client build-server install clean run

build: build-client build-server

build-client:
	go build -o toofan .

build-server:
	cd race_server && go build -o ../toofan-race-server .

install:
	go install .

clean:
	rm -f toofan toofan-race-server

run:
	go run .
