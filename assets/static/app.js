document.getElementById('shorten-form').addEventListener('submit', async (e) => {
    e.preventDefault();

    const urlInput = document.getElementById('url-input').value;
    const resultDiv = document.getElementById('result');
    const shortUrlLink = document.getElementById('short-url');
    const errorP = document.getElementById('error');

    // エラーと結果をリセット
    resultDiv.classList.add('hidden');
    errorP.classList.add('hidden');
    errorP.textContent = '';

    try {
        const response = await fetch('http://localhost:8080/shorten', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ url: urlInput }),
        });

        const data = await response.json();

        if (response.ok) {
            shortUrlLink.textContent = data.short_url;
            shortUrlLink.href = data.short_url;
            resultDiv.classList.remove('hidden');
        } else {
            errorP.textContent = data.error || 'Failed to shorten URL';
            errorP.classList.remove('hidden');
        }
    } catch (err) {
        errorP.textContent = 'Network error occurred';
        errorP.classList.remove('hidden');
    }
});

document.getElementById('copy-btn').addEventListener('click', () => {
    const shortUrl = document.getElementById('short-url').textContent;
    navigator.clipboard.writeText(shortUrl).then(() => {
        alert('Copied to clipboard!');
    }).catch(() => {
        alert('Failed to copy');
    });
});
