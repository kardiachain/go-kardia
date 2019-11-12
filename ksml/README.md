# Kardia smart contract language (KSML)

## Introduction

KSML is used to define actions among dual nodes and kardiachain through watcher actions and dual actions.

KSML is mainly built based on Common Expression Language (CEL), we wrap it with our built-in function.

Read the following for more information [https://github.com/google/cel-spec](https://github.com/google/cel-spec)

## 1. Dual Message, Event Message, Watcher Action and Dual Action

Before going deeper, we need to understand what is event message, dual message and how watcher action and dual action work.

- Dual Message is a message sent from client nodes (NEO, ETH, TRX, etc.) to its dual nodes (KAI-NEO, KAI-ETH, KAI-TRX, etc.)
Dual Message is sent whenever there is a trigger smart contract transaction occurred in client nodes. 
Dual node then reads dual message, find watcher action, trigger action and then send an event to kardiachain with followed dual action.
You can refer dual message structure [here](https://github.com/kardiachain/go-kardia/blob/master/dualnode/protos/DualMessage.proto) 

- Event Message contains information that will be used in ksml as `message` variable. 
Structure of Event Message can be referred [here](https://github.com/kardiachain/go-kardia/blob/master/ksml/proto/Message.proto)

- Watcher action is defined to watch a specific method in a smart contract address. 
A watcher action is caught in handling saved transaction in kardiachain or `Dual Message` received in dual node 
which is sent from client node (ETH, TRX or NEO). When watcher action is detected, 
its actions will be triggered and all results will be stored in `EventMessage.Params`.
Then an event which contains event message and followed dual actions is created and sent to Kardiachain.
EventMessage in Watcher action can be generated in 2 ways:
    - With Dual Node, EventMessage is generated from DualMessage.
    - With KardiaChain, EventMessage is generated from handling transaction.
    - Note that, in watcher action, create transaction is not available.

- Dual action is triggered when dual node's proposer receives event and handle it in `SubmitTx` function.
Dual action can create transaction or publish `KARDIA_CALL` message to client node.

## 2. Structure

- KSML is a list of string which is defined in `${..}`. For example:

    ```
    ${fn:var(blockNumber,uint64,fn:currentBlockHeight())}
    ${fn:validate(blockNumber > 1,SIGNAL_CONTINUE,SIGNAL_STOP)}
    ${smc:trigger(addOrder,message.params[0],message.params[1])}
    "hello"
    ```

- The above strings will be executed sequentially top-down: 

    - First of all, current block height will be get and stored into variable name `blockNumber`
    
    - A validate function will validate if `blockNumber` is greater than 1 or not. If it is not, then stop the sequence. Otherwise, continue.
    
    - If the sequence is continued then it triggers `addOrder` in master smart contract with 2 arguments get in `EventMessage.Params`
    
    - Transaction hash returned from `smc:trigger` will be added to `params` - a global variables. 
    
    - After trigger smart contract, return `hello` into `params`. To return a single, simply add a string like that without `${...}` block.

### 2.1 Parser

Parser reads and executes actions. For more examples about parser, refer [here](https://github.com/kardiachain/go-kardia/blob/master/ksml/tests/parser_test.go)

### 2.2 Global Variables

- `message`: is EventMessage, to use message's attribute, lower case the first character. eg: `message.params`, `message.contractAddress`

- `proxyName`: name of proxy executes actions (NEO, ETH, TRX)

- `contractAddress`: master smart contract address

- `params`: all results returned from CEL without assign in any variable will be appended into params.
    
    - Note: params in `function`, `forEach` or `If` statements cannot access outside params.
    
### 2.3 Built-in functions

There are 2 types built-in functions: 
- `smc` is used to get data or trigger transactions from smart contract
- `fn` is utils functions such as `split`, `replace`, `forEach`, `if`

#### 2.3.1 smc

- getData: get data from smart contract address. 
Returned types will be based on `abi` types. 
Input's params must also follow data types defined in `abi`
    ```
    ${smc:getData(methodName, params...)}
    ```
  
- trigger: if a function is changing smc contract data, 
use this function to trigger it and create transaction hash.
    ```
    ${smc:trigger(methodName, params...)}
    ```
 
#### 2.3.2 fn

- **currentTimeStamp**: get current timeStamp 
    
    ```
    ${fn:currentTimeStamp()}
    ```
  
- **currentBlockHeight**: get current blockHeight of kardiachain

    ```
    ${fn:currentBlockHeight()}
    ```

- **validate**: validate a statements, if it's returns second param, otherwise return third param.
    
    - There are 3 types of params can be used in second and third params:
        
        - SIGNAL_CONTINUE: if parser receives it, it will continue other actions sequentially.
        - SIGNAL_STOP: if parser receives it, it will stop the execution immediately and return an error to prevent dual node create events.
        - SIGNAL_RETURN: if parser receives it, it will stop the execution immediately and return results to EventMessage.Params and let dual node create events.
    - syntax:
        
        ```
        ${fn:validate(statement, true_signal, false_signal)}
        ```
- **var**: define a variable and save it into `parser.userDefinedVariables`

    ```
    ${fn:var(varName, varType, varValue)}
    ```

- **if**:
    
    - Syntax:
    ```
    ${fn:if(conditionName,condition)}
    ...
    ${fn:elif(conditionName,condition)}
    ...
    ${fn:else(conditionName)}
    ...
    ${fn:endif(conditionName)}
    ```
    - An `if`statement must defined its name, and the name must be unique in its scope.
    - It also must contains `endif` with its name as the end.
    - Cannot use `params` outside each block. Returned params will be appended to parent block.
    - Variables defined within these blocks cannot be used outside unless they have already defined outside.
    
- **forEach**:
    
    - Syntax:
    ```
    ${fn:forEach(nameOfForEach,vars,indexVar)}
    ...
    ${fn:endForEach(nameOfForEach)}  
  ```
    - A `forEach`statement must defined its name, and the name must be unique in its scope.
    - It also must contains `endForEach` with its name as the end.
    - Cannot use `params` outside each block. Returned params will be appended to parent block.
    - Variables defined within these blocks cannot be used outside unless they have already defined outside.
    
- **split**: split a string into a list
    - Syntax:
    ```${fn:split(strVar,separator)}```
- **replace**: replace a string in an given string with another string
    - Syntax:
    ```${fn:replace(strVar,oldStrVar,newStrVar)}```
- **defineFunc**: define a function
    - Syntax:
    ```
  ${fn:defineFunc(functionName,params...)}
  ...
  ${fn:endDefineFunc(functionName)}
    ```
  - A `defineFunc`statement must defined its name, and the name must be unique in its scope.
  - It also must contains `endDefineFunc` with its name as the end.
  - Cannot use `params` outside each block. Returned params will be appended to parent block.
  - Variables defined within these blocks cannot be used outside unless they have already defined outside.
  
- **call**: call a defined function
    - Syntax: ```${fn:call(functionName,params...)}```

- **publish**: publish trigger message as KARDIA_CALL topic to client chain
    - Syntax:
    ```
  ${fn:publish(contractAddress,methodName,[params...],[callbacks...])}
  ```
  - `contractAddress`, `methodName` and `params` are used to create triggerMessage.
  - `callbacks` is a list of triggerMessages which is used to send back to kardiachain to trigger smart contracts as DUAL_CALL message.
  - Each `callback` is a triggerMessage (contractAddress, methodName, params) without callbacks.
- **cmp**: compare 2 variable to know whether they are equal or not. If it is equal, return third param, otherwise return fourth param
    
    ```${fn:cmp(var1,var2,trueResult,falseResult)}```

- **mul**: multiply 2 big.Int or big.Float variables.
    ```${fn:mul(fn:int(param1),fn:int(param2))}```
- **div**: do the divide between 2 big.Int or big.Float variables

    ```${fn:div(fn:float(param1),fn:float(param2))}```
- **int**: cast a number into big.Int

    ```${fn:int(aNumber)}```
- **float**: cast a number into big.Float

    ```${fn:float(aNumber)}```
- **exp**: do the exp with 2 int params

    ```${fn:exp(param1,param2)}```

- **format**: format a big.Float with number decimal after `.` sign

    ```${fn:format(floatVar, numberOfDecimalAfterPoint)}```
    
- **round**: round a float number

    ```${fn:round(floatNumber)}```
    

For more example, please refer [here](https://github.com/kardiachain/go-kardia/blob/master/ksml/tests/built_in_test.go)
    






