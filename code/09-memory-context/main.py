"""Memory and the context window.

Replays visual/09-memory-context.html. The window is a list of messages; a
fake compaction keeps only a short summary. The cache check compares bytes.
Run: python3 main.py
"""
RULE = "tests always run in the sandbox, never against the real home folder"
TOOLS_AND_SKILLS = "system prompt, tools, skills"


def fake_summarise(turns):
    return "set up tests, fixed the build"   # the rule did not make it in


def fake_agent(window):
    return "runs tests in the sandbox" if RULE in window else "runs tests against the real home"


# panel:start
class Session:
    def __init__(self, prefix, memory):
        self.prefix = prefix                         # @1,8 stable, sent first
        self.turns = []                              # @2
        self.memory = memory                         # @6 files, outside the window

    def add(self, turn):
        self.turns.append(turn)                      # @2

    def compact(self):                               # @3
        self.turns = [fake_summarise(self.turns)]    # @3 the rule may be lost here

    def recall(self, topic):                         # @7
        self.turns += [f for f in self.memory if topic in f]  # @7 matching files only

    def window(self):
        return "\n".join([self.prefix] + self.turns)  # @1,2


def save_fact(memory, fact, why):                    # @6
    memory.append(f"{fact}. Why: {why}")             # @6 one file per fact


def cache_read(previous, current):                   # @4
    n = 0
    for a, b in zip(previous, current):              # @4 byte for byte, from the start
        if a != b:                                   # @4 one change ends the match
            break
        n += 1
    return n                                         # @4


def build_prefix(stable, changing, changing_first=False):
    if changing_first:
        return changing + "\n" + stable              # @5 suppose the time goes first
    return stable + "\n" + changing                  # @5 changing text goes last
# panel:end


def main():
    print("-- tab 1 --")
    memory = []
    s = Session(TOOLS_AND_SKILLS, memory)
    s.add("turn 1"); s.add("turn 2"); s.add(RULE)
    for i in range(4, 21):
        s.add(f"turn {i}: build output")
    print("  before compaction:", fake_agent(s.window()))
    s.compact()
    print("  after compaction:", s.turns, "->", fake_agent(s.window()))
    save_fact(memory, RULE, "a test once wrote into the real home")
    next_day = Session(TOOLS_AND_SKILLS, memory)
    next_day.recall("sandbox")
    print("  next day, recalled:", fake_agent(next_day.window()))
    in_prefix = Session(TOOLS_AND_SKILLS + "\n" + RULE, memory)
    in_prefix.compact()
    print("  in the prefix, after compaction:", fake_agent(in_prefix.window()))

    print("\n-- tab 2 --")
    stable = TOOLS_AND_SKILLS * 100
    a, b = "time 12:00:01", "time 12:00:02"
    print("  time first: bytes read from cache:", cache_read(build_prefix(stable, a, True), build_prefix(stable, b, True)), "of", len(stable) + len(a) + 1)
    print("  time last: bytes read from cache:", cache_read(build_prefix(stable, a), build_prefix(stable, b)), "of", len(stable) + len(a) + 1)


if __name__ == "__main__":
    main()
