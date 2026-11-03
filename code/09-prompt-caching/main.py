"""Prompt caching: pay once for the stable prefix.

Replays visual/09-prompt-caching.html. Standard library only, no network.
The provider cache is simulated: it keys each start of the prefix by a hash of
its exact bytes, so one changed byte stops the match there. Token counts are
example sizes (text length / 4). Run: python3 main.py
"""
import hashlib

SYSTEM = "s" * 8000                     # about 2,000 tokens
TOOLS = ["graph_query", "meta_schema", "meta_help", "wallet_balance"]
TOOL_TEXT = 3000                        # characters per tool description
SKILLS = "k" * 16000                    # about 4,000 tokens


def tok(text):
    return len(text) // 4


def tool_block(names):
    return "".join(n.ljust(TOOL_TEXT, ".") for n in names)


# panel:start
TTL = 300                                                # @8 seconds an unused entry lives


def build_prefix(tools, skills, sort_tools=True):        # @1 the stable part
    if sort_tools:                                       # @9
        tools = sorted(tools)                            # @9 same order, same bytes
    return [("system", SYSTEM), ("tools", tool_block(tools)), ("skills", skills)]  # @1,2 breakpoint after skills


class ProviderCache:
    def __init__(self):
        self.expires = {}                                # prefix hash -> expiry time

    def call(self, prefix, tail, now):                   # @3
        read = written = 0
        matching, seen = True, b""
        for name, text in prefix:
            seen += text.encode()
            key = hashlib.sha256(seen).hexdigest()       # @4 the exact bytes so far
            if matching and self.expires.get(key, -1) >= now:  # @4 found, not expired
                read += tok(text)                        # @5 cheap and fast
            else:
                matching = False                         # @6 the match stops here
                written += tok(text)                     # @6 full price, then cached
            self.expires[key] = now + TTL                # @6,8 each use renews it
        fresh = sum(tok(t) for t in tail)                # @7 history and turn, always fresh
        return read, written, fresh
# panel:end


def show(label, r):
    print(f"{label}: read from cache {r[0]:,}, processed and written {r[1]:,}, fresh {r[2]:,} tokens")


def main():
    print("-- tab 1: same prefix, cache hit --")
    sizes = {name: tok(text) for name, text in build_prefix(TOOLS, SKILLS)}
    print(f"prefix: system {sizes['system']:,} + tools {sizes['tools']:,} + skills {sizes['skills']:,}"
          f" = {sum(sizes.values()):,} tokens (example sizes)")
    cache, t = ProviderCache(), 0
    history = ""
    for n in range(1, 21):
        history += "h" * 4000
        r = cache.call(build_prefix(TOOLS, SKILLS), [history, "n" * 800], t)
        if n <= 2 or n == 20:
            show(f"call {n} at {t} s", r)
        last = t
        t += 30
    print(f"last use at {last} s, entries expire at {last + TTL} s (TTL {TTL} s)")
    t += 3600
    show(f"after lunch, call at {t} s", cache.call(build_prefix(TOOLS, SKILLS), [history, "n" * 800], t))

    print("\n-- tab 2: one change breaks the match --")
    cache = ProviderCache()
    show("call 1", cache.call(build_prefix(TOOLS, SKILLS, sort_tools=False), ["", "n" * 800], 0))
    reordered = list(reversed(TOOLS))     # the MCP server reconnected
    print(f"the same {len(TOOLS)} tools, reversed:", reordered)
    show("call 2, tools in a new order", cache.call(build_prefix(reordered, SKILLS, sort_tools=False), ["", "n" * 800], 30))
    cache = ProviderCache()
    cache.call(build_prefix(TOOLS, SKILLS), ["", "n" * 800], 0)
    show("call 2, a skill edited", cache.call(build_prefix(TOOLS, SKILLS + " edited"), ["", "n" * 800], 30))
    cache = ProviderCache()
    cache.call(build_prefix(TOOLS, SKILLS), ["", "n" * 800], 0)
    show("fix, sorted tools, call 2 reordered", cache.call(build_prefix(reordered, SKILLS), ["", "n" * 800], 30))


if __name__ == "__main__":
    main()
