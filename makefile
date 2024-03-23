gim: gim.go
	go build gim.go
	./gim gim.go

run:
	./gim gim.go

clean:
	-rm -rf gim