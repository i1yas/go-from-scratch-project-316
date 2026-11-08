package engine

func isRetriableStatus(statusCode int) bool {
	return statusCode == 492 || statusCode >= 500
}
