"""Swarm handoff: peers pass the conversation.

Replays visual/09-swarm-handoff.html. Standard library only, no network.
Each agent's routing decision is a fake model: a keyword rule read from its
handoff tool descriptions. Run: python3 main.py
"""

# what each agent believes the others own (its handoff tool descriptions)
OLD_RULES = {
    "triage": [("top-up", "billing"), ("refund", "billing"), ("who funded", "investigation")],
    "billing": [("who funded", "investigation"), ("query", "investigation")],
    "investigation": [("refund", "billing"), ("top-up", "billing")],
}
NEW_RULES = dict(OLD_RULES, investigation=[("refund", "billing"), ("top-up", "billing")],
                 billing=[("who funded", "investigation")])  # refunds for failed queries: billing
ANSWERS = {
    "billing": "billing: the top-up is waiting for one more block",
    "investigation": "investigation: funded by 0x18aa4b2c3d4e5f60718293a4b5c6d7e8f90acb24 and "
                     "0x77f1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f730ff",
}


def fake_route(agent, message, rules):
    """Stands in for the model choosing a handoff tool, or none."""
    for word, target in rules[agent]:
        if word in message.lower():
            return target
    return agent


def answer(agent, message):
    if agent == "billing" and "refund" in message.lower():
        return "billing: refund of the failed query approved"
    return ANSWERS.get(agent, f"{agent}: answered")


# panel:start
CAP = 2                                                  # @6 handoffs per user turn


def handle(message, holder, rules, cap=CAP, log=print):  # @1
    handoffs = 0                                         # @2
    while True:                                          # @3
        target = fake_route(holder, message, rules)      # @4,10 its own rule decides
        if target == holder:                             # @5 it owns the topic
            return holder, answer(holder, message)       # @5
        if cap is not None and handoffs == cap:          # @6 cap reached
            return "triage", "triage: answered the query part, asked a human for the refund"  # @7
        log(f"  handoff {handoffs + 1}: {holder} -> {target}, summary: '{message}'")  # @8
        holder = target                                  # @8 the user is not asked again
        handoffs += 1                                    # @9
        if cap is None and handoffs == 4:                # no cap: stop the demo at 4
            return None, "no answer after 4 handoffs"
# panel:end


def main():
    print("-- tab 1: handed to the right peer --")
    holder, reply = handle("My wallet top-up failed", "triage", OLD_RULES)
    print(" ", reply)
    holder, reply = handle("Who funded 0xabc?", holder, OLD_RULES)
    print(" ", reply)

    print("\n-- tab 2: ping-pong --")
    msg = "Refund me for a query that failed"
    print(" with no cap:")
    print(" ", handle(msg, "triage", OLD_RULES, cap=None)[1])
    print(" with the cap of 2:")
    print(" ", handle(msg, "triage", OLD_RULES)[1])
    print(" with clear descriptions (refunds for failed queries: billing):")
    print(" ", handle(msg, "triage", NEW_RULES)[1])


if __name__ == "__main__":
    main()
