## Exchanger实现方案

### 模块能力

- 提供启停错误恢复，保证服务适配器与目标适配器的交易有序性；
- 提供跨链交易事件消费能力，正确处理跨链交易过程中的服务的请求、回执、回滚等任务；

### 数据结构
1. mode 
   - 描述网关模式
2. srcChainId， srcBxhId 
   - 描述服务对象的应用链id，中继链id。大规模下则没有srcChainId
3. srcAdapt 
   - 描述网关服务对象（中继/直连模式下的应用链，大规模下的中继链）
4. destAdapt 
   - 为srcAdapt提供对等服务的适配器
5. adapt0ServiceMeta 
   - 初始化为从srcAdapt获取的服务meta信息
6. adapt1ServiceMeta
   - 初始化为从destAdapt获取的服务meta信息

### 启动流程

- 从srcAdapt中获取可用服务列表
  - 中继/直连模式下获取应用链的可用服务列表
  - 大规模模式下获取托管中继链中该中继链注册服务之外的所有服务列表
- 分别从两个adapt中填充服务列表下的serviceMeta信息
- 执行recover流程抹平srcAdapt和destAdapt的ServiceMeta
- 启动双端ibtp消费

### 恢复流程（以中继模式为例：src-应用链，dest-中继链）

1. 处理src -> dest
   1. 遍历应用链Adapt下Interchain的所有服务，处理两种遗漏交易：
      - ①应用链作为来源链未发出的请求；
      - ②应用链作为目的链未发出的回执；
   2. 抹平index差距中的所有ibtp后将dest的InterchainCounter和SourceReceiptCounter与src对齐。
2. 处理dest -> src
   1. 遍历中继链Adapt下Interchain的所有服务，处理三种遗漏交易：
      - ①应用链作为目的链未收到的请求；
      - ②应用链作为来源链未收到的回执；
      - ③应用链作为来源链未收到的rollback；
   2. 抹平index差距中的所有ibtp后将src的SourceInterchainCounter和ReceiptCounter与dest对齐
3. 最终得到两份一致的ServiceMeta。
4. 在消费过程中两边ServiceMeta不需要保证一致，仅由ibtp发送成功后触发维护。

### 恢复流程（union模式：src-中继链，dest-union_pier）
1. src获取所有**非当前**中继链的服务interchain；
2. 处理src -> dest
   1. 遍历中继链Adapt下Interchain的所有服务，处理两种遗漏交易：
      - ①中继链作为来源链未发出的请求；
      - ②中继链作为目的链未发出的回执；
   2. 抹平index差距中的所有ibtp后将dest的SourceInterchainCounter和ReceiptCounter与src对齐。
3. 处理dest -> src
   1. 遍历unionAdapt下Interchain的所有服务，处理两种遗漏交易：
      - ①中继链作为目的链未收到的请求；
      - ②中继链作为来源链未收到的回执；
   2. 抹平index差距中的所有ibtp后将src的InterchainCounter和SourceReceiptCounter与dest对齐
4. 最终得到两份一致的ServiceMeta。
5. 在消费过程中两边ServiceMeta不需要保证一致，仅由ibtp发送成功后触发维护。

### 消费流程

1. 消费srcAdapt生产的ibtp
   1. 监听ibtp后识别它的from链是否为srcAdapt的（应用链/中继链）id，这里中继/直连模式下ibtp.from.chainId和srcChainId匹配，
      大规模模式下ibtp.from.bxhId和srcBxhId匹配。分为两种情况：
      - 匹配正确：则为srcAdapt作为来源链发出的请求交易；
      - 匹配失败：则为srcAdapt作为目的链发出的回执信息。
   2. 获取该ibtp交易服务对下对应的srcAdapt的index信息；
   3. 对比ibtp所在的index和exchanger维护的index，分为三种情况:
      - index >= ibtp.index   : 抛弃交易，该交易已被执行
      - index < ibtp.index -1 : 处理遗失交易，传递**区间信息、交易方向、服务对、是否请求**这四种参数
      - index = ibtp.index -1 : 提交ibtp至destAdapt，成功后抹平ibtp.index和index信息

1. 消费destAdapt生产的ibtp
   1. 监听ibtp后识别它的from链是否为srcAdapt的（应用链/中继链）id，这里中继/直连模式下ibtp.from.chainId和srcChainId匹配，
      大规模模式下ibtp.from.bxhId和srcBxhId匹配。分为两种情况：
      - 匹配正确：则为srcAdapt作为来源链收到的回执信息 或 作为来源链收到的rollback信号；
      - 匹配失败：则为srcAdapt作为目的链收到的请求交易。
   2. 获取该ibtp交易服务对下对应的destAdapt的index信息；
   3. 对比ibtp所在的index和exchanger维护的index，分为三种情况:
      - index >= ibtp.index   : 抛弃交易，该交易已被执行
      - index < ibtp.index -1 : 处理遗失交易，传递**区间信息、交易方向、服务对、是否请求**这四种参数
      - index = ibtp.index -1 : 提交ibtp至srcAdapt，成功后抹平ibtp.index和index信息
