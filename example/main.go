package main

import (
	"context"
	"fmt"
	client "github.com/explainableaixai/aitoolsblocklist-go"
	"os"
)

func main() {
	c := client.New(os.Getenv("AQ_API_KEY"))
	result, err := c.Check(context.Background(), "chat.openai.com")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", result)
}
