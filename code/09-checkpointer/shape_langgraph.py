"""LangGraph shape only. Not run: it needs langgraph and a Postgres server.
The runnable version of the same steps is main.py."""
from langgraph.checkpoint.memory import MemorySaver           # tests only
from langgraph.checkpoint.postgres import PostgresSaver       # production
from langgraph.types import Command, interrupt


def human_approval(state):
    answer = interrupt({"question": "approve this report?"})  # @5 pause, state saved
    return {"approved": answer}                               # @4


with PostgresSaver.from_conn_string(DB_URL) as saver:         # @8 durable checkpoints
    app = graph.compile(checkpointer=saver)
    config = {"configurable": {"thread_id": "inv-42"}}        # @1
    app.invoke({"wallet": address}, config)                   # @3 runs up to the interrupt
    # Later, in a new process:
    app.invoke(Command(resume="the analyst"), config)         # @2,9 loads the last checkpoint
    history = list(app.get_state_history(config))             # @10 every checkpoint, for a rewind
