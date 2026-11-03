"""LangChain shape of the same parser. Shape only: not run, needs LangChain."""
from typing import Literal

from langchain.output_parsers import RetryWithErrorOutputParser
from langchain_core.output_parsers import PydanticOutputParser
from pydantic import BaseModel, Field


class Verdict(BaseModel):  # @4,5 the schema checks each field
    risk_kind: Literal["mixer", "exchange", "scam", "none"]  # @4
    confidence: float = Field(ge=0, le=1)  # @4


parser = PydanticOutputParser(pydantic_object=Verdict)  # @3,4,5
prompt = "Screen this wallet.\n" + parser.get_format_instructions()  # @1
text = model.invoke(prompt).content  # @2
verdict = parser.parse(text)  # @3,4,5 raises OutputParserException

retry = RetryWithErrorOutputParser.from_llm(parser=parser, llm=model, max_retries=2)  # @6,7

typed = model.with_structured_output(Verdict)  # @9 the provider constrains the tokens
