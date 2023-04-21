package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/henomis/lingoose/decoder"
	llmmock "github.com/henomis/lingoose/llm/mock"
	"github.com/henomis/lingoose/memory/ram"
	"github.com/henomis/lingoose/pipeline"
	"github.com/henomis/lingoose/prompt"
)

func main() {

	cache := ram.New()

	llm1 := &llmmock.LlmMock{}
	prompt1 := prompt.New("Hello how are you?")
	pipe1 := pipeline.NewStep("step1", llm1, pipeline.LlmModeCompletion, prompt1, decoder.NewDefaultDecoder(), cache)

	myout := &struct {
		First  string
		Second string
	}{}
	llm2 := &llmmock.JsonLllMock{}
	prompt2, _ := prompt.NewPromptTemplate(
		`It seems you are a random word generator. Your message '{{.output}}' is nonsense. 
		Anyway I'm fine {{.value}}!`,
		map[string]string{
			"value": "thanks",
		},
	)
	pipe2 := pipeline.NewStep("step2", llm2, pipeline.LlmModeCompletion, prompt2, myout, decoder.NewJSONDecoder(), cache)

	var values []string
	regexDecoder := decoder.NewRegExDecoder(`(\w+)\s(\w+)\s(.*)`)
	prompt3, _ := prompt.NewPromptTemplate(
		`Oh! It seems you are a random JSON word generator. You generated two strings, 
		first:'{{.First}}' and second:'{{.Second}}'. {{.value}}\n\tHowever your first 
		message was: '{{.step1.output}}'`,
		map[string]string{
			"value": "Bye!",
		},
	)
	pipe3 := pipeline.NewStep("step3", llm1, pipeline.LlmModeCompletion, prompt3, values, regexDecoder, cache)

	pipelineSteps := pipeline.New(
		pipe1,
		pipe2,
		pipe3,
	)

	response, err := pipelineSteps.Run(nil)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("Final output: %s\n", strings.Join(response.([]string), ", "))
	fmt.Println("---Memory---")
	dump, _ := json.MarshalIndent(cache.All(), "", "  ")
	fmt.Printf("%s\n", string(dump))
}
