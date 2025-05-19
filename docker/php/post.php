<?php
declare(strict_types=1);

// Enable error reporting for development
ini_set('display_errors', '1');
error_reporting(E_ALL);

// Set content type
header('Content-Type: text/plain');

// Debug logging
error_log("POST request received");
error_log("Raw input: " . file_get_contents('php://input'));
error_log("POST data: " . print_r($_POST, true));
error_log("FILES data: " . print_r($_FILES, true));
error_log("Request method: " . $_SERVER['REQUEST_METHOD']);
error_log("Content type: " . $_SERVER['CONTENT_TYPE']);
error_log("Content length: " . $_SERVER['CONTENT_LENGTH']);

$length = 0;
$stat = "PASSED";

try {
    // Parse raw input if needed
    if (empty($_POST) && !empty($_SERVER['CONTENT_LENGTH'])) {
        $rawInput = file_get_contents('php://input');
        error_log("Parsing raw input: " . $rawInput);
        parse_str($rawInput, $_POST);
        error_log("Parsed POST data: " . print_r($_POST, true));
    }

    if (count($_POST) || count($_FILES)) {
        error_log("Processing POST/FILES data");
        foreach ($_POST as $key => $val) {
            error_log("Processing POST key: $key, value: $val");
            $md5 = md5($val);
            if ($key !== $md5) {
                $stat = "FAILED";
                error_log("MD5 mismatch: $md5 != $key");
                echo "server:err {$md5} != {$key}\n";
            }
            $length += strlen($key) + strlen($val);
        }

        foreach ($_FILES as $key => $file) {
            error_log("Processing FILE: " . print_r($file, true));
            if ($file["error"] === UPLOAD_ERR_OK) {
                $tmp = $file["tmp_name"];
                if (!file_exists($tmp)) {
                    error_log("Temporary file not found: $tmp");
                    throw new RuntimeException("Temporary file not found: {$tmp}");
                }
                
                $md5 = md5_file($tmp);
                error_log("File MD5: $md5, expected key: $key");
                if ($key !== $md5) {
                    $stat = "FAILED";
                    error_log("File MD5 mismatch: $md5 != $key");
                    echo "server:err {$md5} != {$key}\n";
                }
                $length += strlen($key) + filesize($tmp);
                
                if (!unlink($tmp)) {
                    error_log("Failed to delete temporary file: $tmp");
                    throw new RuntimeException("Failed to delete temporary file: {$tmp}");
                }
            } else {
                $stat = "FAILED";
                error_log("File upload error: " . $file["error"]);
                echo "server:file err " . $file["error"] . "\n";
            }
        }

        error_log("Total data length: $length");
        echo "server:got data length {$length}\n";
    } else {
        error_log("No POST or FILES data received");
    }
} catch (Throwable $e) {
    $stat = "FAILED";
    error_log("Error: " . $e->getMessage());
    echo "server:error " . $e->getMessage() . "\n";
}

$response = "-{$stat}-POST(" . count($_POST) . ") FILE(" . count($_FILES) . ")\n";
error_log("Sending response: $response");
echo $response;

// Ensure all output is flushed
if (ob_get_level()) {
    ob_end_flush();
}
flush();

// Log completion
error_log("Request processing completed");