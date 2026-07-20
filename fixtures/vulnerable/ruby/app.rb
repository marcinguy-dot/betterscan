# Brakeman / OpenGrep style: command injection and SQL-like string concat
require 'open3'

def run(cmd)
  # dangerous system with user input
  system("ls #{cmd}")
  `#{cmd}`
end

def find_user(id)
  # SQL injection via string interpolation
  query = "SELECT * FROM users WHERE id = '#{id}'"
  query
end

def render(name)
  # XSS-ish raw output pattern
  "<h1>#{name}</h1>"
end
