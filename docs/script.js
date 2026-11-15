'use strict';
document.querySelectorAll('[data-copy]').forEach((button) => {
  button.addEventListener('click', async () => {
    const text = document.getElementById(button.dataset.copy).textContent;
    try {
      await navigator.clipboard.writeText(text);
      button.textContent = 'Copied!';
      document.getElementById('copy-status').textContent = 'Command copied to clipboard.';
      window.setTimeout(() => { button.textContent = 'Copy'; }, 2000);
    } catch {
      document.getElementById('copy-status').textContent = 'Copy unavailable. Select the command and copy it manually.';
      button.textContent = 'Select text';
      const range = document.createRange();
      range.selectNodeContents(document.getElementById(button.dataset.copy));
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    }
  });
});
