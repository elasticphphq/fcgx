<?php

ini_set("display_errors", 1);
header("Content-Type: text/plain");

$length = 0;
$stat = "PASSED";

if (count($_POST) || count($_FILES)) {
    foreach ($_POST as $key => $val) {
        $md5 = md5($val);
        if ($key !== $md5) {
            $stat = "FAILED";
            echo "server:err {$md5} != {$key}\n";
        }
        $length += strlen($key) + strlen($val);
    }

    foreach ($_FILES as $key => $file) {
        if ($file["error"] === UPLOAD_ERR_OK) {
            $tmp = $file["tmp_name"];
            $md5 = md5_file($tmp);
            if ($key !== $md5) {
                $stat = "FAILED";
                echo "server:err {$md5} != {$key}\n";
            }
            $length += strlen($key) + filesize($tmp);
            unlink($tmp);
        } else {
            $stat = "FAILED";
            echo "server:file err " . file_upload_error_message($file["error"]) . "\n";
        }
    }

    echo "server:got data length {$length}\n";
}

echo "-{$stat}-POST(" . count($_POST) . ") FILE(" . count($_FILES) . ")\n";

function file_upload_error_message($error_code) {
    return match ($error_code) {
        UPLOAD_ERR_INI_SIZE => 'The uploaded file exceeds the upload_max_filesize directive in php.ini',
        UPLOAD_ERR_FORM_SIZE => 'The uploaded file exceeds the MAX_FILE_SIZE directive in the HTML form',
        UPLOAD_ERR_PARTIAL => 'The uploaded file was only partially uploaded',
        UPLOAD_ERR_NO_FILE => 'No file was uploaded',
        UPLOAD_ERR_NO_TMP_DIR => 'Missing a temporary folder',
        UPLOAD_ERR_CANT_WRITE => 'Failed to write file to disk',
        UPLOAD_ERR_EXTENSION => 'File upload stopped by extension',
        default => 'Unknown upload error',
    };
}