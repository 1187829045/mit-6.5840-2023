### 实验要求

构建一个MapReduce系统。实现一个调用应用程序的Map和Reduce函数并处理文件读写的worker进程，以及一个分配任务给worker并处理
worker故障的协调器进程。 实现类似于MapReduce论文中的系统。


任务是实现一个分布式MapReduce系统，包括两个程序：协调器（coordinator）和worker。
协调器进程有且仅有一个，worker进程可以有多个并行执行。在实际系统中，worker会运行在不同的机器上，但本次实验中所有worker将在同一台机器上运行。
worker通过RPC与协调器通信，每个worker请求任务、读取任务输入文件、执行任务并写入输出文件。
协调器应监测worker是否在合理时间内完成任务（本实验中设定为10秒），若超时则将任务分配给其他worker。

### 开始
sudo PATH=$PATH:/usr/local/go/bin bash test-mr.sh
这个命令运行测试是否正确


协调器和worker的"main"例程位于main/mrcoordinator.go和main/mrworker.go；请勿修改这些文件。
你可以在mr/coordinator.go、mr/worker.go和mr/rpc.go中实现你的代码。

需要补充实现mr目录下的代码


### 规则

Map 阶段应将中间键分配到为 nReduce 个 Reduce 任务创建的桶中，其中 nReduce 是 Reduce 任务的数量 ——
由 main/mrcoordinator.go 传递给 MakeCoordinator() 的参数。每个 Mapper 应创建 nReduce 个中间文件，供 Reduce 任务使用。

Worker 应将第 X 个 Reduce 任务的输出放入文件 mr-out-X。

mr-out-X 文件应包含每个 Reduce 函数输出的一行。行格式应为 Go 的 "%v %v" 格式，使用键和值进行调用。
参考 main/mrsequential.go 中注释为“这是正确的格式”的行。如果实现格式与此偏差过大，测试脚本将失败。
您可以修改 mr/worker.go、mr/coordinator.go 和 mr/rpc.go。可以临时修改其他文件进行测试，但确保代码可在原始版本上运行；我们将使用原始版本进行测试。

Worker 应将中间 Map 输出放在当前目录中的文件中，以便后续 Reduce 任务读取。
main/mrcoordinator.go 期待 mr/coordinator.go 实现一个 Done() 方法，当 MapReduce 任务完成时返回 true；此时 mrcoordinator.go 将退出。
任务完成时，Worker 进程应退出。实现此功能的简单方法是使用 call() 的返回值：如果 Worker 未能联系到 Coordinator，可以认为 Coordinator 已退出，
即任务完成，因此 Worker 也可以终止。根据设计，还可以创建一个 "please exit" 的伪任务，Coordinator 可将其分配给 Worker 来要求退出。

### 提示

先实现一个简单的轮廓：先修改 mr/worker.go 中的 Worker()，让其向 Coordinator 发送一个 RPC 请求以请求任务。然后修改 Coordinator，返回一个尚未开始的 Map 任务的文件名。
接着修改 Worker，读取该文件并调用应用的 Map 函数（在 mrsequential.go 中）。






### 思路
首先 你只需要看五个文件 mrcoordinator.go mrwork.go 以及mr目录下的三个go程序即可

思路，work先向协调节点请求任务，协调节点分配任务后应该记录开始执行的时间，并在后面根据当前时间减去开始时间判断是否有超时，
word得到任务后根据任务类型分别执行，比如map或者reduce任务，map任务应该首先应该读取文件的内容并调用map函数，得到KV键值对，
然后排序后，将相同键值对聚集在一起比如 key=‘a',values ="11111111"，然后根据key进行hash看会映射到那个reduce并生成中间文件。mr-X-Y,当map阶段结束后
，应该变为reduce阶段，并把Y=当前reduce节点ID的值放入一个数组中作为reduce任务的输入，reduce任务应该统计好数量即可并将生成数据格式化放入mr-out-Y
文件中，当reduce任务完成后应该更新为Done状态，这样协调节点发现是Done状态就会直接break







