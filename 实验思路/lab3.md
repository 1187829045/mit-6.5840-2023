# Lab3

## 观看论文和笔记的实验相关部分

### zoonkeeper
1. **服务器处理重复请求的合理方式是**，服务器会根据请求的唯一号或者其他的客户端信息来保存一个表。这样服务器可以记住，
哦，我之前看过这个请求，并且执行过它，我会发送一个相同的回复给它，因为我不想执行相同的请求两次。例如，假设这是一个
写请求，你不会想要执行这个请求两次。所以，服务器必须要有能力能够过滤出重复的请求。第一个请求的回复可能已经被网络丢
包了。所以，服务器也必须要有能力能够将之前发给第一个请求的回复，再次发给第二个重复的请求。所以，服务器记住了最初的
回复，并且在客户端重发请求的时候将这个回复返回给客户端。如果服务器这么做了，那么因为服务器或者Leader之前执行第一
个读请求的时候，可能看到的是X=3，那么它对于重传的请求，可能还是会返回X=3。所以，我们必须要决定，这是否是一个合法的行为。

2. 你们在实验中会完成这样的机制，服务器发现了重复的请求，并将之前的回复重新发给客户端。这里的问题是，服务器最初在这里看到
了请求，最后回复的数据是本应在之前一个时间点回复的数据，这样是否合理？我们使用线性一致的定义的一个原因是，它可以用来解
释问题。例如，在这个场景里面，我们可以说，这样的行为符合线性一致的原则。

3. 如果你有一个读请求，例如Lab3中的get请求，把它发给某一个副本而不是Leader。如果我们这么做了，对于写请求没有什么帮助，是
我们将大量的读请求的负担从Leader移走了。现在对于读请求来说，有了很大的提升，因为现在，添加越多的服务器，我们可以支持越多的客
户端读请求，因为我们将客户端的读请求分担到了不同的副本上。 所以，现在的问题是，如果我们直接将客户端的请求发送给副本，我们能得到
预期的结果吗？是的，实时性是这里需要考虑的问题。Zookeeper作为一个类似于Raft的系统，如果客户端将请求发送给一个随机的副本，那
个副本中肯定有一份Log的拷贝，这个拷贝随着Leader的执行而变化。假设在Lab3中，这个副本有一个key-value表，当它收到一个读X的请
求，在key-value表中会有X的某个数据，这个副本可以用这个数据返回给客户端。所以，功能上来说，副本拥有可以响应来自客户端读请求的所有数据
。这里的问题是，没有理由可以相信，除了Leader以外的任何一个副本的数据是最新（up to date）的。这里有很多原因导致副本没有最新的数据，
其中一个原因是，这个副本可能不在Leader所在的过半服务器中。对于Raft来说，Leader只会等待它所在的过半服务器中的其他follower对于
Leader发送的AppendEntries消息的返回，之后Leader才会commit消息，并进行下一个操作。所以，如果这个副本不在过半服务器中，它或许永
远也看不到写请求。又或许网络丢包了，这个副本永远没有收到这个写请求。所以，有可能Leader和过半服务器可以看见前三个请求，但是这个副本只
能看见前两个请求，而错过了请求C。所以从这个副本读数据可能读到一个旧的数据。所以，如果这里不做任何改变，并且我们想构建一个线性一致的
系统，尽管在性能上很有吸引力，我们不能将读请求发送给副本，并且你也不应该在Lab3这么做，因为Lab3也应该是线性一致的。这里是线性一致阻止
了我们使用副本来服务客户端

4. Zookeeper并不要求返回最新的写入数据。Zookeeper的方式是，放弃线性一致性。它对于这里问题的解决方法是，不提供
线性一致的读。所以，因此，Zookeeper也不用为读请求提供最新的数据。它有自己有关一致性的定义，而这个定义不是线
性一致的，因此允许为读请求返回旧的数据。所以，Zookeeper这里声明，自己最开始就不支持线性一致性，来解决这里的
技术问题。如果不提供这个能力，那么（为读请求返回旧数据）就不是一个bug。这实际上是一种经典的解决性能和强一致之
间矛盾的方法，也就是不提供强一致。

5. Zookeeper的确有一些一致性的保证，用来帮助那些使用基于Zookeeper开发应用程序的人，来理解他们的应用程序，以及
理解当他们运行程序时，会发生什么。与线性一致一样，这些保证与序列有关。Zookeeper有两个主要的保证.第一个是，写请求
是线性一致的.第二个是任何一个客户端的请求，都会按照客户端指定的顺序来执行，论文里称之为FIFO（First In First Out）客户端序列。




### CRAQ 

https://keys961.github.io/2020/05/03/%E8%AE%BA%E6%96%87%E9%98%85%E8%AF%BB-CRAQ/


## 实验要求

### 简介
在本实验中，您将使用Lab2的Raft库构建一个容错的键值存储服务。您的键值服务将是一个复制的状态机，由多个使用Raft进行复制的键值服务器组成。只要大多数
服务器存活并能通信，您的键值服务就应继续处理客户端请求，即使存在其他故障或网络分区。完成Lab 3后，您将实现Raft交互图中所示的所有组件（客户端、服务
和Raft）。客户端可以向键值服务发送三种RPC：Put(key, value)、Append(key, arg)和Get(key)。该服务维护一个简单的键值对数据库。键和值都是字符
串类型。Put(key, value)替换数据库中特定键的值，Append(key, arg)将参数追加到键的值末尾，Get(key)获取键的当前值。对于不存在的键，Get应返回空
字符串。对不存在的键执行Append应等效于Put操作。

每个客户端通过Clerk（包含Put/Append/Get方法）与服务交互，Clerk管理与服务器的RPC通信。您的服务必须确保客户端对Clerk的Get/Put/Append方法的
调用是线性化的。如果按顺序逐个调用，这些方法的行为应如同系统只有单一状态副本，每个调用都能观察到前序调用对状态的修改。对于并发调用，返回值和最终状态
必须与这些操作按某种顺序逐个执行的结果相同。如果调用在时间上重叠（例如客户端X调用Clerk.Put()，客户端Y调用Clerk.Append()，然后客户端X的调用返回
），则视为并发调用。调用必须观察到所有在调用开始前已完成调用的效果。线性化特性对应用程序非常方便，因为它呈现了单服务器顺序处理请求的行为。例如，如果
一个客户端收到更新请求的成功响应，后续其他客户端的读取操作必须能看到该更新的效果。对单服务器来说实现线性化相对容易，但对复制服务则更具挑战性：所有服
务器必须为并发请求选择相同的执行顺序，必须避免使用过时状态响应客户端，且必须在故障恢复后保留所有已确认的客户端更新。

本实验分为两部分。Part A中，您将基于Raft实现键值服务（不使用快照）。Part B中，您将使用Lab 2D的快照实现，使Raft能丢弃旧日志条目。

### 环境准备
我们在src/kvraft中提供了骨架代码和测试。您需要修改kvraft/client.go、kvraft/server.go以及可能的kvraft/common.go。


### Part A：无快照的键值服务（中等/困难）
每个键值服务器（"kvserver"）对应一个Raft节点。客户端将Put/Append/Get RPC发送给Raft领导者节点。服务器代码将这些操作提交到Raft日志，
所有kvserver按日志顺序执行操作，维护相同的键值数据库副本。客户端可能不知道当前领导者。如果发错服务器或无法连接，客户端应重试其他服务器。
如果操作被提交到Raft日志（即应用到状态机），领导者通过RPC响应结果。若操作未提交（例如领导者变更），服务器返回错误，客户端重试其他服务器。

kvserver间不应直接通信，只能通过Raft交互。 首要任务是实现无丢包、无故障场景下的解决方案。

您需要：

在client.go的Clerk方法中添加RPC发送代码,在server.go实现PutAppend()和Get() RPC处理,定义描述操作的Op结构体（在server.go）服务器通过
applyCh执行已提交的Op命令,RPC处理器需在Raft提交Op后响应,当通过"One client"测试时完成此阶段。

注意事项：

调用Start()后需等待Raft完成共识，通过applyCh获取已提交命令,注意kvserver与Raft库间的死锁问题 ,可在ApplyMsg/RPC中添加字段（通常不需要）
Get()必须通过Raft日志保证不返回过期数据 ,尽早添加锁，使用go test -race检查竞态条件,处理网络故障和重复请求（通过唯一标识操作）,处理领导者变更场景（客户端需重试新领导者）
Clerk应缓存最后已知领导者 ,通过操作去重确保仅执行一次



#### 实现思路

1. 客户端远程调用服务端的函数,并且客户端这边维护一个服务端的领导者Id
2. 客户端优先领导者Id的方法,不是领导者再轮训去找到领导者
3. 服务端实现GET,PUT,Append方法,其中一个map存储值,一个map用于检测请求是否已来过,一个待读管道用来接收,raft提交时(向applyChan放入命令),发出的命令
4. 服务端启动协成检测这个applyCha管道,从中读取命令,放入待读管道中,当被调用了Get等方法时,就是从这个等待管道中,判断是否已提交日志的,提交成功返回成功,超时.客户端就重试


### Part B：带快照的键值服务（困难）

当前实现未使用Raft快照，重启时需要重放完整日志。现需修改kvserver，当Raft状态超过maxraftstate大小时，调用Snapshot()保存快照。maxraftstate=-1表示禁用快照。

#### 关键点：

1. 比较persister.RaftStateSize()与maxraftstate阈值
2. 适时调用Raft快照
3. 重启时从persister.ReadSnapshot()恢复状态
4. 快照需包含去重所需的状态信息
5. 快照中的结构体字段需大写
   
#### 总建议：

1. Lab 3测试应在400秒实际时间/700秒CPU时间内完成
2. TestSnapshotSize应少于20秒实际时间

#### 实验思路

1. 生成快照主要是保存 数据和唯一性判断map,生成快照后调用lab2的快照传递函数
2. 生成快照要判断容量是否达到生成快照的界限