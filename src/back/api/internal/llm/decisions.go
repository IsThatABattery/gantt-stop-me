package llm

import "context"

// Question is one choice question for a decision model.
type Question struct {
	Name         string   `json:"name"` // how the answer refers to it
	Instructions string   `json:"instructions"`
	Choices      []Choice `json:"choices"`
}

// Choice is one value a Question can be answered with.
type Choice struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Answer is a decision model's answer to one Question.
type Answer struct {
	Type          string        `json:"type"` // "choice", or "refusal" when the model declined
	Name          string        `json:"name"` // the Question's
	Choice        string        `json:"choice"`
	Probabilities []Probability `json:"probabilities"`
	Confidence    float64       `json:"confidence"`
}

// Probability is how likely the model finds one Choice's value.
type Probability struct {
	Value       string  `json:"value"`
	Probability float64 `json:"probability"`
}

// Decide asks model, a decision model, every question about input in one POST {BaseURL}/decisions, and returns the
// answers by question name. A refusal, or any other answer that is not a choice, is left out, so it reads as missing.
func (c *Client) Decide(ctx context.Context, model, input string, questions []Question) (map[string]Answer, error) {
	type choiceQuestion struct {
		Type string `json:"type"`
		Question
	}
	in := struct {
		Model     string           `json:"model"`
		Input     string           `json:"input"`
		Questions []choiceQuestion `json:"questions"`
	}{Model: model, Input: input, Questions: make([]choiceQuestion, len(questions))}
	for i, q := range questions {
		in.Questions[i] = choiceQuestion{"choice", q}
	}
	var out struct {
		Answers []Answer `json:"answers"`
	}
	if err := c.post(ctx, "/decisions", in, &out); err != nil {
		return nil, err
	}
	answers := map[string]Answer{}
	for _, a := range out.Answers {
		if a.Type == "choice" {
			answers[a.Name] = a
		}
	}
	return answers, nil
}
