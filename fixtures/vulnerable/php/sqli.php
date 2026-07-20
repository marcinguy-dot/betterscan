<?php
// OpenGrep classic: SQL injection + command injection + XSS
$id = $_GET['id'];
$query = "SELECT * FROM users WHERE id = " . $id;
mysqli_query($conn, $query);

$cmd = $_GET['cmd'];
system("ping -c 1 " . $cmd);
echo $_GET['name'];
?>
