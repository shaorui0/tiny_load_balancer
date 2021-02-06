# 背景

回到最初的起点，我是否要用go来写？是不是太慢了？一个陌生的语言，开发效率很显然会变慢，并且对我的面试也没什么帮助！

1. load balancer在一个分布式系统中很重要。[ref](http://www.aosabook.org/en/distsys.html#fig.distsys.1)
2. 作为一个分布式系统的组件，让我更熟悉相关领域的东西
    - 反向代理是什么
    - 哪些地方要加锁
    - lb是如何工作的，哪些地方需要注意？
3. 我不需要策略非常完善，有一个简单的RR就可以了。这样日后，我也能提出一些『优化点』为我所用
    - 哪些优化点？
        1. balance 的策略
        2. 加权
        3. select -> poll -> epoll
4. 同时增加了我写golang的经验
    - 一些锁的场景
    - goroutine的场景，可以让我简单的熟悉一下goroutine（当然，只是简单的使用了一些）
5. 是**build-your-own-xxx**上面一个非常值得写的小项目

# 设计目标

1. 能够与我 python 写的 web server 合起来正常工作。以后再完善其他的组件（比如weibo）。
2. 能够简单的进行负载均衡（作为一个组件这样就足够了）
    - 能够完成基本的reverse proxy的功能，进行 request/response 的成功中转
    - 能够找到 next server
    - 能够正确的（health）找到 next server
    - 能够保证足够多的request找到 next server

##  实现的功能

1. 其中是有一个 `response handler(request)` 这样一个功能的函数签名。需要有一个**反向代理**去转发request然后接受response（*这就是nginx的作用？*）
    - 作者说明一个可复用的接口，写代码之前，如何使用这个接口是我要知道的（TODO 如何测试？）
2. 能够找到 next index
    - 在分布式中，这个过程好像叫『选举』（select），我这里的算法是简单的RR
3. 能够进行对 next index server 进行健康检查 --- 转发的目的地是否有用？  
    - 作者提供了两种可行的方案。显然，我需要对这两种 healthCheck 方案进行设计（这样未来才好说这块）
        - 消极 passive 检查
            - 每次过来一个request，检查一下（会不会太慢）
        - 积极 active 检查
            - 一个子进程/线程/goroutine 去检查，每隔多少秒
                - 检查多少次？什么时候结束检查，并把它标记为错误？
                - 如何来标记检查？**context**？这是一个成熟的解决方案。
4. 能够保证足够多的 request 过来能够正常的工作（需要保证『并发』的场景）
    - 锁？还是其他？（原子操作）



# 设计思路及折衷

1. 关于选举，算法如果现在选择RR，未来他应该是独立的。如果我要换一个算法，我希望是有一个接口（`getNext`）。
2. 关于健康检查，有积极和消极两种方式，目前我是希望都实现。不过仍然需要对比一下 TODO .
    - 积极的通过子进程去检查有没有什么问题？即使我标记了 next 为 true，仍然有可能为false（相当于是错误处理）
    - 很显然，不仅仅是对next检查，而是对所有的（all）都进行检查
    - 消极的话，试错成本是不是比较大（time）？
    - 当然，health 本身，发生的比例应该是不大的，检查能够过滤掉一些，然后消极的尝试（context：retry...）是在积极之后，两者似乎并不冲突。
3. 


TODO 还有哪些设计思路？折衷才是这里要考虑的

1. 一切都是理所当然吗？作者设计了这样的思路我就要这样做吗？又能改进的地方吗？
    - 我需要考虑哪些方面，
    - 数据结构的设计，作者用了context，这个

## reverse proxy

[example: nginx + flask](https://stackoverflow.com/questions/62945229/nginx-reverse-proxy-for-python-app-using-docker-compose)

1. 调研了一下，python 好像没有这个现成的东西？
2. 这里的reverse proxy 怎么工作的？所谓的request 和 response 其实是对消息的一个http封装（header + ...）
3. 有没有比较好的现成的


TODO 其实可以改改之前 python 写的那个 web server ，稍微改改就可以作为自己的 reverse proxy 


# 系统设计

TODO checklist 《代码大全》


## 基本介绍

高并发的 client 访问 lb ，回访问临界区 `nextIndex` ，这个 index 在 pool 里面？
request 获得到了相匹配的server之后，需要』『尝试』发送消息，消息不一定能传到（可能有错误返回，也可能直接返回结果（一个封装好的rp））
最后 response 被返回给相对应的client

## 系统流程图及说明



如何测试？我还不太知道该怎么测试 reverse proxy ， 这种涉及到网络的东西，我希望能够进行mock。


### 相关的一些功能

数据类型基本就是http定义的 request + response

lb:
- get request 
- return response
- raise Exception('access failed.')

reverse proxy:
- send request to backend
- get response from backend
- TODO 具体 reverse proxy 怎么使用还要看


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



### 错误恢复、处理、错误信息保存（不单单是log，甚至trace级别的？）

1. log要做好
2. 每个阶段验证数据是否正常
    - 正常发送
    - 正常接收
    - 异常接收（error、exception）


### 吞吐量

1. 首先，没有lb的测试结果
2. 有lb的测试结果
    - 少于五个backend
    - 多余五个backend



### health check 反应时间

1. 主动检查，每二十秒钟一次
2. 被动检查，每三秒钟一次，超过三次即为失败


### 用户唯一关心的就是能接受到正常的 response

1. backend可以返回 `response(backend_name)` 作为验证
2. 大量的吞吐情况下，如何更大程度的减少瓶颈（原子操作 + 锁带来的损失）


## 与外部系统的接口



外部需要开一（多）个 server，client 通过浏览器或者 command line 模拟。
request 作为一个概念是如何通过socket传递给 lb 最终到达 Server 的。
我需要留什么样的接口？其实就是一个头一个尾，这样的接口是怎样的？
我看作者是有一个可直接使用的接口，配上自己写一个`func lb(request, response)`


### 输入

格式：request
来源：作为server，肯定是监听某个 ip + port


### 输出

格式：response
来源：作为backend的client
- 选择backend ==> ip + port
- 转发request
- 获取response，并转发response


### 通信界面

如何使用？通过什么样的方式，配置信息？在main里面，

1. 手动设置lb的url
2. 参数设置几个备用的backend
3. 通过http访问lb，看看能否正常转发（各个地方log）

```
// 使用方式
go main.go {backend_1_url} {backend_2_url} {backend_3_url}
```

### 每个部件都可以独立进行测试？

1. rp的测试方式。本质就是作为一个client
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

### 模块定义是否清楚了

这个交互过程还是很清晰的，部件不多，整体的代码其实也不算多，只是一个比较小的项目，但是足够我学习的

### 是否有可能的改动？

1. 目前一切基本都是确定的
2. 其实相互之间的接口还是比较模糊，关于参数是什么、返回值是什么？还是没有定义好

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

#### passive

1. 超过 retry
2. 超过 attempt

#### active 
1. 直接设置为False

### select

1. all of them cannot run normally.



## 优化点

1. 本质也是一种server，所以涉及到select之类的io reuse，可以升级为epoll
2. 配置化
3. 选举策略更『智能』