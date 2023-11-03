package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/joho/godotenv"

	openai "github.com/sashabaranov/go-openai"
)

const defaultRegion = "us-east-1"

const basePrompt = "あなたは答えが1つには決まらない問題に対して、一つの視点を提供するアドバイザーとしての役割を持っています。なので、以下の質問に対して、必ず最初に「賛成」または「反対」のどちらかの立場を表明してください。最初に賛成であるか反対であるかを明示し、その後に理由を述べてください。\n"

const askClaude2 = "あなたは答えが1つには決まらない問題に対して、一つの視点を提供するアドバイザーとしての役割を持っています。あなたは以下の問題に対して{final_answer}であるという立場を動かさずに、以下の2人の賢者の意見を参考にし、尚且つ自身の意見も踏まえて「賛成」または「反対」のどちらかの立場を明確にしてください。賢者Aの意見: GPT-3.5 Turbo,賢者Bの意見: Claude Instant"

const claudePromptFormat = "\n\nHuman: %s\n\nAssistant:"

var brc *bedrockruntime.Client

var client *openai.Client

func init() {

	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("読み込み出来ませんでした: %v", err)
	}

	// OpenAI API Client
	client = openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	// AWS Bedrock Runtime Client
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = defaultRegion
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region), config.WithSharedConfigProfile(os.Getenv("AWS_PROFILE")))
	if err != nil {
		log.Fatal(err)
	}

	brc = bedrockruntime.NewFromConfig(cfg)
}

func main() {
	var gpt35Answer string
	var claudeInstantAnswer string

	// プロンプトの入力
	fmt.Print("Enter your prompt: ")
	var userPrompt string
	fmt.Scanln(&userPrompt)

	var wg sync.WaitGroup
	wg.Add(2)

	// GPT-3.5 Turboにプロンプトを送信（並行処理）
	go func() {
		defer wg.Done()
		fmt.Println("Sending to GPT-3.5 Turbo...")
		gpt35Answer = sendGPT35(userPrompt)
		fmt.Println("GPT-3.5 Turbo is expressed!!!")
	}()

	// Claude Instantにプロンプトを送信（並行処理）
	go func() {
		defer wg.Done()
		fmt.Println("Sending to Claude Instant...")
		claudeInstantAnswer = sendClaudeInstantV1(userPrompt)
		fmt.Println("Claude Instant is expressed!!!")
	}()

	// 並行処理が完了するまで待つ
	wg.Wait()

	// "..." の表示をクリア
	fmt.Print("\033[2K\r")

	fmt.Println("--------------------------------------------------")
	fmt.Println("GPT-3.5 Turbo: \n", gpt35Answer)
	fmt.Println("--------------------------------------------------")
	fmt.Println("Claude Instant: \n", claudeInstantAnswer)
	fmt.Println("--------------------------------------------------")
	fmt.Println("Awaiting final opinions...")

	// 2秒間の待機
	time.Sleep(2 * time.Second)

	// 2つの回答をClaude2にまとめてもらう
	_, err := sendClaude2(gpt35Answer, claudeInstantAnswer)
	if err != nil {
		log.Fatal("Error sending to Claude2: ", err)
	}
}

// GPT-3.5 Turboにプロンプトを送信
func sendGPT35(prompt string) string {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: basePrompt + prompt,
				},
			},
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	return resp.Choices[0].Message.Content
}

// Claude Instantにプロンプトを送信
func sendClaudeInstantV1(prompt string) string {
	payload := Request{Prompt: fmt.Sprintf(claudePromptFormat, basePrompt+prompt), MaxTokensToSample: 2048}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	output, err := brc.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		Body:        payloadBytes,
		ModelId:     aws.String("anthropic.claude-instant-v1"),
		ContentType: aws.String("application/json"),
	})

	if err != nil {
		log.Fatal(err)
	}

	var resp Response

	err = json.Unmarshal(output.Body, &resp)

	if err != nil {
		log.Fatal(err)
	}

	return resp.Completion
}

func sendClaude2(answer1, answer2 string) (string, error) {
	combinedPrompt := fmt.Sprintf("GPT-3.5 Turbo: %s\n\nClaude Instant: %s", answer1, answer2)

	payload := Request{Prompt: fmt.Sprintf(claudePromptFormat, askClaude2+combinedPrompt), MaxTokensToSample: 2048}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	output, err := brc.InvokeModelWithResponseStream(context.Background(), &bedrockruntime.InvokeModelWithResponseStreamInput{
		Body:        payloadBytes,
		ModelId:     aws.String("anthropic.claude-v2"),
		ContentType: aws.String("application/json"),
	})

	if err != nil {
		return "", err
	}

	resp, err := processStreamingOutput(output, func(ctx context.Context, part []byte) error {
		fmt.Print(string(part))
		return nil
	})

	if err != nil {
		log.Fatal("streaming output processing error: ", err)
	}

	return resp.Completion, nil
}

type StreamingOutputHandler func(ctx context.Context, part []byte) error

func processStreamingOutput(output *bedrockruntime.InvokeModelWithResponseStreamOutput, handler StreamingOutputHandler) (Response, error) {

	var combinedResult string
	resp := Response{}

	for event := range output.GetStream().Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:

			//fmt.Println("payload", string(v.Value.Bytes))

			var resp Response
			err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&resp)
			if err != nil {
				return resp, err
			}

			handler(context.Background(), []byte(resp.Completion))
			combinedResult += resp.Completion

		case *types.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)

		default:
			fmt.Println("union is nil or unknown type")
		}
	}

	resp.Completion = combinedResult

	return resp, nil
}

// request/response model
type Request struct {
	Prompt            string   `json:"prompt"`
	MaxTokensToSample int      `json:"max_tokens_to_sample"`
	Temperature       float64  `json:"temperature,omitempty"`
	TopP              float64  `json:"top_p,omitempty"`
	TopK              int      `json:"top_k,omitempty"`
	StopSequences     []string `json:"stop_sequences,omitempty"`
}

type Response struct {
	Completion string `json:"completion"`
}
