1. data structure
    - backend
    - backend pool

2. What aspects are worth thinking about?
    - reverse proxy
    - race condition
        - atom add
        - lock
            - mutex
            - rwlock
    - check backend status
        - active check
            - select
        - passive check
            - goroutine
    - context
        - error retry
        - attempts