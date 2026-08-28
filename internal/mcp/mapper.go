package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/genai"
)

func ToGenAITools(mcpTools []mcp.Tool) []*genai.Tool {
	if len(mcpTools) == 0 {
		return nil
	}
	
	var funcs []*genai.FunctionDeclaration
	for _, t := range mcpTools {
		// A simple conversion for the schema.
		// genai.Schema expects specific types, we will just pass it as an object
		// and use a simplified version, or we can marshal/unmarshal it if needed.
		
		schema := &genai.Schema{
			Type: genai.TypeObject,
			Properties: make(map[string]*genai.Schema),
		}

		if t.InputSchema.Properties != nil {
			propsMap := t.InputSchema.Properties
			for k, v := range propsMap {
				propDict, ok := v.(map[string]interface{})
				if ok {
						tStr, _ := propDict["type"].(string)
						var gType genai.Type
						switch tStr {
						case "string":
							gType = genai.TypeString
						case "integer":
							gType = genai.TypeInteger
						case "number":
							gType = genai.TypeNumber
						case "boolean":
							gType = genai.TypeBoolean
						case "array":
							gType = genai.TypeArray
						case "object":
							gType = genai.TypeObject
						default:
							gType = genai.TypeString
						}
						
						desc, _ := propDict["description"].(string)
						
						schema.Properties[k] = &genai.Schema{
							Type: gType,
							Description: desc,
						}
					}
				}
			}

		funcs = append(funcs, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		})
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: funcs,
		},
	}
}

// ResultToMap converts CallToolResult to a map for genai
func ResultToMap(res *mcp.CallToolResult) map[string]any {
	if res == nil {
		return nil
	}
	// We just grab the first text content for simplicity in the POC
	var text string
	for _, c := range res.Content {
		if txt, ok := c.(mcp.TextContent); ok {
			text += txt.Text + "\n"
		}
	}
	return map[string]any{
		"result": text,
	}
}
