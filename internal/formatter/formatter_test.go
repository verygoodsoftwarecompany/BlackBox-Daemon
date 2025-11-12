package formatter

import (
"testing"
"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/emitter"
)

func TestNewFormatterChain(t *testing.T) {
chain := NewFormatterChain()
if chain == nil {
t.Fatal("Expected non-nil formatter chain")
}
if len(chain.formatters) != 0 {
t.Errorf("Expected 0 formatters in new chain, got %d", len(chain.formatters))
}
}

func TestDefaultFormatter(t *testing.T) {
formatter := NewDefaultFormatter()
if formatter.Name() != "default" {
t.Errorf("Expected formatter name 'default', got '%s'", formatter.Name())
}
}

func TestJSONFormatter(t *testing.T) {
formatter := NewJSONFormatter()
if formatter.Name() != "json" {
t.Errorf("Expected formatter name 'json', got '%s'", formatter.Name())
}
}

func TestCSVFormatter(t *testing.T) {
formatter := NewCSVFormatter()
if formatter.Name() != "csv" {
t.Errorf("Expected formatter name 'csv', got '%s'", formatter.Name())
}
}

func TestCreateFormatterChain(t *testing.T) {
formatters := []string{"default"}
emitterConfigs := []emitter.EmitterConfig{
{
Type: "file",
Config: map[string]interface{}{
"path": "/tmp/test.log",
},
},
}

chain, err := CreateFormatterChain(formatters, emitterConfigs)
if err != nil {
t.Fatalf("Expected no error creating chain, got %v", err)
}
defer chain.Close()

if len(chain.formatters) != 1 {
t.Errorf("Expected 1 formatter, got %d", len(chain.formatters))
}
}
