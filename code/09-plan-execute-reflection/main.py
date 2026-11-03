"""Plan and execute, then reflect.

Replays visual/09-plan-execute-reflection.html. The planner, executors and
critic are fake functions; the checks and the round cap are real code.
Run: python3 main.py
"""
TABLES = {"core_token_transfers"}


def fake_planner(goal, fix=None):
    table = fix or "core_transfers"          # the first plan guessed a wrong name
    return [f"task {i}" for i in range(1, 3)] + [f"migrate {table}"] + [f"task {i}" for i in range(4, 9)]


def fake_executor(task):
    return task


def check(task):
    if task.startswith("migrate"):
        return task.split()[1] in TABLES
    return True


def fake_critic(work, fresh, round_no, empty_test):
    if not fresh:
        return []                            # it trusts its own reasoning
    if empty_test and round_no == 1:
        return ["task 6 is a test that asserts nothing"]
    return [] if round_no >= 2 or empty_test else ["test misses an empty input", "runbook link is broken"]


# panel:start
ROUND_CAP = 3                                     # @7


def run_plan(goal):
    plan = fake_planner(goal)                     # @1 every step before any work
    done, i = [], 0
    while i < len(plan):                          # @2
        task = fake_executor(plan[i])             # @3 cheaper model, one task
        if check(task):                           # @4
            done.append(task)
            i += 1
        else:                                     # @5
            print(f"  check failed: {task}")      # @5
            plan = done + fake_planner(goal, fix="core_token_transfers")[i:]  # @5 replan the rest
    return done


def review(work, fresh=True, empty_test=False):
    for round_no in range(1, ROUND_CAP + 1):      # @7
        findings = fake_critic(work, fresh, round_no, empty_test)  # @6 fresh context, a rubric
        print(f"  round {round_no}: {len(findings)} findings {findings}")
        if not findings:                          # @8
            return "pass"                         # @8
        work = work + ["fixed: " + f for f in findings]  # @8 an executor fixes them
    return "stopped at the round cap"             # @7
# panel:end


def main():
    print("-- tab 1 --")
    done = run_plan("the change")
    print(f"  {len(done)} tasks done, task 3: {done[2]}")
    print("  critic:", review(done))
    print("\n-- tab 2 --")
    print("  the author reviews itself:", review(done, fresh=False, empty_test=True))
    print("  a fresh critic:", review(done, fresh=True, empty_test=True))


if __name__ == "__main__":
    main()
