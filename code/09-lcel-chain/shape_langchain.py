"""LangChain LCEL shape of the same chain. Shape only: not run, needs LangChain."""
from langchain_anthropic import ChatAnthropic
from langchain_core.output_parsers import JsonOutputParser
from langchain_core.prompts import ChatPromptTemplate

prompt = ChatPromptTemplate.from_template("Summarise this incident log: {log}")  # @1,4
model = ChatAnthropic(model="claude-sonnet-5-5")  # @1,5
parser = JsonOutputParser()  # @1,6

chain = prompt | model | parser  # @2,7 a RunnableSequence

result = chain.invoke({"log": log})  # @8 {"summary": ..., "risk": "high"}
results = chain.batch(logs)  # @3 30 logs at once
for chunk in chain.stream({"log": log}):  # @3
    print(chunk)
safe = chain.with_retry(stop_after_attempt=2)  # @3

# tracing: set LANGSMITH_TRACING=true and every runnable  # @9
# records its input, output and error as a span           # @9,10
