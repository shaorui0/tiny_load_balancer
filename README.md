# 背景

## 为什么要做这个项目？

1. [熟悉分布式系统相关领域，load balancer 在分布式系统中的重要性](http://www.aosabook.org/en/distsys.html#fig.distsys.1)
    - **反向代理**是什么
    - 分布式锁怎么做？
    - lb如何工作，哪些方面需要注意？

2. 不需要策略非常完善，简单的RR（round-robin）即可。日后进行优化 ---> 哪些潜在的优化点？
    1. balance 的策略
        - 加权
    2. argv ---> config file

3. 增加对语言的熟悉程度
    - sync/lock
    - http/http_util/http_test
    - reverse proxy


# 设计目标

1. 能够简单的进行负载均衡（作为一个组件足够了）
    - 【基本功能】  能够完成基本的 reverse proxy 的功能，进行 request/response 的转发。
    - 【正确】     能够正确的（health）找到 next server
    - 【高并发】   能够保证足够多的 request 找到 next server

##  实现的功能


### 调研

1. handler 的形式： `func handler(writer, request)`。
2. reverse proxy 存在现成的API： `httputil.NewSingleHostReverseProxy(serverUrl)`
3. 错误处理（转发不成功）： `proxy.ErrorHandler(writer, request, error)`
4. find next backend
    - get next index
        - `atomAdd()`
    - check backend is health
        - `bool isAlive`
        - `setAliveByBackend()`
        - `setAliveByUrl()`
3. healthCheck
    - static check
        - isAlive
        - run checking function in backend (goroutine)
    - ErrorHandler
        - context
            - retry
            - attempt

4. high concurrency
    - RWLock
    - atom add



# 设计思路及折衷

1. 关于选举，目前是RR，未来可升级（比如加权重）。
2. 关于健康检查，有积极和消极两种方式.
    - 通过子进程在 backend 进行检查。(for all backend)
    - 通过 retry 对同一个backend进行重试，通过attempts 轮询不同的backend，直到找到正常运行的backend，或 attempts > 3。

## reverse proxy

[Example: nginx + flask](https://stackoverflow.com/questions/62945229/nginx-reverse-proxy-for-python-app-using-docker-compose)


# 系统设计



## 基本介绍

- 高并发的 client 访问 lb ，会访问临界区 `current`，需要加lock。
- request 获得到了相匹配的server之后，需要『尝试』发消息，消息不一定能传到（可能有错误返回）

## 系统流程图及说明


### 相关的一些功能

http struct: request + response

lb:
- get request 
- return response
- raise Exception('access failed.')

reverse proxy:
- send request to backend
- get response from backend
- handler func

get_next_index:
- get current index from pool
- (atom) set next index (+1) 

health check
- active
    - while True, i += 1, i %= len(backend), check backend[i]
    - set status
    - check next backend
- passive
    - get backend object
    - access backend
    - (get retry number from context)
    - if failed, retry
    - if retry more than three time, set False
    - TODO attempt, how to use it?





## check list

> 《code complete 2》

### 1. 与外部系统的接口

parse url of backend server from command line.


##### 输入

格式：request
来源：作为server，肯定是监听某个 ip + port


##### 输出

格式：response
来源：作为backend的client
- 选择backend ==> ip + port
- 转发request
- 获取response，并转发response


### 2. 错误恢复、处理、错误信息保存

1. log要做好
2. 每个阶段验证数据是否正常
    - 正常发送
    - 异常发送（error、exception）
    - 正常接收
    - 异常接收（error、exception）


### 3. 吞吐量

TODO test, 1 + n



### 4. time about health checking

1. 主动检查, 10 millseconds
2. 被动检查, 10 minutes


### 5. 用户唯一关心的就是能接受到正常的 response

1. backend可以返回 `response(backend_name)` 作为验证
2. TODO 大量的吞吐情况下，如何更大程度的减少瓶颈（原子操作 + 锁带来的损失）


### 6. 独立测试？

1. rp 的测试方式。本质就是作为一个client
2. get_next:
    - 测试其原子性
3. set_alive/get_alive
    - 测试其同步性
4. 测试数据结构设计的正确性
5. health check
    - retry次数过多怎么办？直接设置为false（这也是我的目的）
5. 对于返回错误的地方
    - 直接终止？
    - 解决问题 + 记录？

### 7. 模块定义是否清楚？

交互过程还是很清晰的，部件不多。

### 8. 是否有可能的改动？

1. 目前一切基本都是确定的
2. 其实相互之间的接口还是比较模糊。关于参数是什么，返回值是什么？还是没有定义好。

## 数据结构及说明

backend:
- url
- is_alive
- reverse_proxy(response, request)


backends:
- a_list_of_backend
- current_backend_index


## 异常处理

### health check

被动访问backend失败

#### active 

1. 超过 retry
2. 超过 attempt

#### passive
1. 直接设置为False
