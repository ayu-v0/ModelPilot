package benchmark

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

func LoadJSONL(path string) (domain.Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return domain.Dataset{}, err
	}
	defer f.Close()
	d := domain.Dataset{Name: path}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 10*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var s domain.Sample
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			return domain.Dataset{}, fmt.Errorf("line %d: %w", line, err)
		}
		if s.ID == "" {
			s.ID = fmt.Sprintf("sample-%d", line)
		}
		d.Samples = append(d.Samples, s)
	}
	if err := scanner.Err(); err != nil {
		return domain.Dataset{}, err
	}
	return d, nil
}
