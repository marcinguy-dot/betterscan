// OpenGrep: SQL injection / command injection in Node
const { exec } = require('child_process');
const mysql = require('mysql');

function searchUsers(req, res) {
  const q = req.query.q;
  // SQL injection via string concat
  const sql = "SELECT * FROM users WHERE name = '" + q + "'";
  connection.query(sql, function (err, rows) {
    res.json(rows);
  });
}

function runCmd(req, res) {
  // command injection
  exec('ls ' + req.query.dir, (err, stdout) => {
    res.send(stdout);
  });
}

module.exports = { searchUsers, runCmd };
