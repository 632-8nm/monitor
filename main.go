package main

func main() {
	server := NewServer()
	server.Start(":8080")
}
