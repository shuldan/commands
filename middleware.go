package commands

// Middleware оборачивает обработчик команды.
type Middleware func(next HandleFunc) HandleFunc

func buildChain(handler HandleFunc, middlewares []Middleware) HandleFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}
