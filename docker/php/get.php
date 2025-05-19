<?php
declare(strict_types=1);

// Enable error reporting for development
ini_set('display_errors', '1');
error_reporting(E_ALL);

// Set content type
header('Content-Type: text/plain');

// Simple response
echo "-PASSED-";