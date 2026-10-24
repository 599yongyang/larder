package ops

import "larder/internal/core/model"

func planTitle(code, fallback string) model.Message {
	return model.Message{Code: code, Fallback: fallback}
}

func planMessage(code, fallback string, params map[string]string) model.Message {
	return model.Message{Code: code, Fallback: fallback, Params: params}
}

func addWarning(plan *model.Plan, code, fallback string, params map[string]string) {
	plan.Warnings = append(plan.Warnings, fallback)
	plan.WarningMessages = append(plan.WarningMessages, planMessage(code, fallback, params))
}
