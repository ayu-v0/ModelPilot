package benchmark

import (
	"github.com/ayu-v0/ModelPilot/internal/domain"
	"github.com/ayu-v0/ModelPilot/pkg/metrics"
)
func Quality(dataset domain.Dataset, results []domain.RequestResult, names []string) map[string]float64 { out:=map[string]float64{}; if len(names)==0{return out}; byID:=map[string]domain.RequestResult{};for _,r:=range results{byID[r.SampleID]=r}; for _,name:=range names{var metric metrics.QualityMetric;switch name{case "exact_match":metric=metrics.ExactMatch{};default:continue};var total float64;var count int;for _,s:=range dataset.Samples{r,ok:=byID[s.ID];if !ok||!r.Success{continue};total+=metric.Score(s,domain.ModelResponse{Output:r.Output});count++};if count>0{out[name]=total/float64(count)}};return out}
}
