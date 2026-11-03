"""Sandboxing: let the agent run code where it cannot hurt.

Replays visual/09-sandboxing.html. This program never touches your real home:
"the real home" here is a temporary folder that plays that part. A temp folder
and an environment are not a real boundary; the container line in
container_command() is what you use in production. Run: python3 main.py
"""
import os
import shutil
import subprocess
import sys
import tempfile

# The "test": a CLI wallet command that writes into $HOME, and looks for keys.
TEST = """
import os, pathlib
home = pathlib.Path(os.environ["HOME"])
(home / ".cli").mkdir(exist_ok=True)
(home / ".cli" / "wallet.json").write_text('{"owner": "test"}')
print("wallet written to", home)
print("keys readable:", (home / ".ssh" / "id_key").exists())
print("API_TOKEN seen:", os.environ.get("API_TOKEN", "none"))
"""


def run_test(env, cwd):
    out = subprocess.run([sys.executable, "-c", TEST], env=env, cwd=cwd,
                         capture_output=True, text=True, timeout=60)
    return out.stdout.strip().split("\n")


# panel:start
def container_command(work):
    return ["docker", "run", "--rm",                  # @2,8 the box is thrown away
            "-v", f"{work}:/work", "-w", "/work",     # @3 only the work folder
            "--tmpfs", "/home/agent",                 # @4 HOME is empty, in memory
            "-e", "HOME=/home/agent",                 # @4
            "--network", "none",                      # @5 or a proxy with an allow list
            "--memory", "2g", "--cpus", "2",          # @7
            "test-image", "python3", "test.py"]       # @1


def run_in_sandbox(work):
    box = tempfile.mkdtemp(prefix="box-")             # @2
    try:
        env = {"HOME": box, "PATH": os.defpath}       # @4,6 no secrets are passed in
        return run_test(env, cwd=work), box           # @1,3,7 limit: 60 s in run_test
    finally:
        shutil.rmtree(box)                            # @8 whatever it did dies here
# panel:end


def main():
    work = tempfile.mkdtemp(prefix="work-")
    real_home = tempfile.mkdtemp(prefix="real-home-")  # plays the real home
    os.mkdir(os.path.join(real_home, ".ssh"))
    open(os.path.join(real_home, ".ssh", "id_key"), "w").close()
    os.mkdir(os.path.join(real_home, ".cli"))
    wallet = os.path.join(real_home, ".cli", "wallet.json")
    open(wallet, "w").write('{"owner": "the user"}')

    print("-- tab 1: inside the sandbox --")
    print("container form:", " ".join(container_command("/path/to/work")))
    lines, box = run_in_sandbox(work)
    print("\n".join(lines))
    print("time limit: 60 s, not hit | box deleted after the run:", not os.path.exists(box))
    print("real wallet after the test:", open(wallet).read())

    print("\n-- tab 2: on the real machine --")
    env = {"HOME": real_home, "PATH": os.defpath, "API_TOKEN": "tok-example"}
    print("\n".join(run_test(env, cwd=work)))
    print("real wallet after the test:", open(wallet).read())

    shutil.rmtree(work)
    shutil.rmtree(real_home)


if __name__ == "__main__":
    main()
