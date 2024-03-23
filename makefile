gim: gim.go
	go build gim.go
	./gim test.txt

run:
	./gim test.txt

clean:
	-rm -rf gim