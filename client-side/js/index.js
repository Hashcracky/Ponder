document.addEventListener("DOMContentLoaded", function() {
    const uploadForm = document.getElementById("upload-form");
    const uploadButton = document.getElementById("upload-button");
    const uploadStatus = document.getElementById("upload-status");
    const downloadForm = document.getElementById("download-form");
    const downloadButton = document.getElementById("download-button");
    const displayButton = document.getElementById("display-button");
    const downloadStatus = document.getElementById("download-status");
    const fileContent = document.getElementById("file-content");
    const logEntries = document.getElementById("log-entries");
    const importForm = document.getElementById("import-form");
    const importButton = document.getElementById("import-button");
    const importStatus = document.getElementById("import-status");


    uploadForm.addEventListener("submit", function(event) {
        event.preventDefault();
        const fileInput = document.getElementById("file-upload");
        const file = fileInput.files[0];
        if (!file) {
            uploadStatus.textContent = "No file selected.";
            return;
        }

        const formData = new FormData();
        formData.append("file", file);

        uploadButton.textContent = "Uploading...";
        fetch("/api/upload", {
            method: "POST",
            body: formData
        }).then(response => response.json()).then(data => {
            uploadStatus.textContent = `Message: ${data.message}, Duration: ${data.duration}`;
            uploadButton.textContent = "Upload";
        }).catch(error => {
            uploadStatus.textContent = "Upload failed.";
            uploadButton.textContent = "Upload";
        });
    });

    downloadForm.addEventListener("submit", function(event) {
        event.preventDefault();
        const numLines = document.getElementById("num-lines").value;
        const substring = document.getElementById("substring").value;
        if (!numLines) {
            downloadStatus.textContent = "Please enter the number of lines.";
            return;
        }

        let url = `/api/download/${numLines}`;
        if (substring) {
            url += `?substring=${encodeURIComponent(substring)}`;
        }

        downloadButton.textContent = "Downloading...";
        fetch(url).then(response => response.text()).then(data => {
            const blob = new Blob([data], { type: 'text/plain' });
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.style.display = 'none';
            a.href = url;
            a.download = 'downloaded_file.txt';
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(url);
            downloadButton.textContent = "Download";
            downloadStatus.textContent = "File downloaded successfully.";
        }).catch(error => {
            downloadStatus.textContent = "Download failed.";
            downloadButton.textContent = "Download";
        });
    });

    displayButton.addEventListener("click", function() {
        const numLines = document.getElementById("num-lines").value;
        const substring = document.getElementById("substring").value;
        const maxLines = 50000;

        if (!numLines) {
            downloadStatus.textContent = "Please enter the number of lines.";
            return;
        }

        if (numLines > maxLines) {
            downloadStatus.textContent = `The number of lines exceeds the display limit of ${maxLines}.`;
            return;
        }

        let url = `/api/download/${numLines}`;
        if (substring) {
            url += `?substring=${encodeURIComponent(substring)}`;
        }

        displayButton.textContent = "Displaying...";
        fetch(url).then(response => response.text()).then(data => {
            if (data.length > 5000000) {
                downloadStatus.textContent = "File too large to display.";
                displayButton.textContent = "Display";
                return;
            }
            fileContent.textContent = data;
            displayButton.textContent = "Display";
        }).catch(error => {
            downloadStatus.textContent = "Display failed.";
            displayButton.textContent = "Display";
        });
    });

    importForm.addEventListener("submit", function(event) {
        event.preventDefault();
        importButton.textContent = "Importing...";
        fetch("/api/import", {
            method: "POST"
        })
        .then(response => response.json())
        .then(data => {
            importStatus.textContent = `Message: ${data.message}, Duration: ${data.duration}`;
            importButton.textContent = "Import";
        })
        .catch(error => {
            importStatus.textContent = "Import failed.";
            importButton.textContent = "Import";
        });
    });

    function fetchLogEntries() {
        fetch("/api/event-log").then(response => response.json()).then(data => {
            logEntries.innerHTML = "";
            const entries = data.log_entries.slice(-250).reverse();
            entries.forEach((entry, index) => {
                const opacity = 1 - (index / 250);
                const logEntry = document.createElement("div");
                logEntry.style.opacity = opacity;
                logEntry.textContent = `${entry.time} - ${entry.event}: ${entry.message}`;
                logEntries.appendChild(logEntry);
            });
        }).catch(error => {
            logEntries.textContent = "Failed to load log entries.";
        });
    }

    fetchLogEntries();
    setInterval(fetchLogEntries, 600000);
});
