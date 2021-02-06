1. data structure
    - backend
    ```go
    is_alive
    url
    mutex, 每个backend / 每个thread 都有自己的lock，对某个公共资源进行加锁
    ReverseProxy # 怎么理解
    ```
    - backend pool
    ```go
    current_index
    a list of backend
    ```

2. What aspects are worth thinking about?
    - reverse proxy
    - race condition
        - [ next_one ] atom add
        - [ server_pool ] lock
            - [ is_alive ]mutex
            - [ set_alive ]rwlock
    - check backend status
        - active check
            - select
            - attemp / retry
        - passive check
            - goroutine
            - eventloop / 20s
    - context
        - error **retry**
        - **attempts**
