"""Bandit / OpenGrep targets: command injection, eval, unsafe YAML."""
import os
import subprocess
import pickle
import yaml


def run_user_cmd(user_input: str) -> None:
    # B602 / B605: shell injection
    os.system("ls " + user_input)
    subprocess.call("echo " + user_input, shell=True)


def eval_user(code: str):
    # B307: use of eval
    return eval(code)


def load_pickle(data: bytes):
    # B301: pickle load
    return pickle.loads(data)


def load_yaml(doc: str):
    # B506: unsafe yaml load
    return yaml.load(doc)
