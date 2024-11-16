// File input enhancement
document.querySelectorAll('input[type="file"]').forEach(input => {
    input.addEventListener('change', () => {
        const file = input.files[0];
        if (file) {
            const button = input.closest('form').querySelector('button');
            button.textContent = `Add ${file.name}`;
        }
    });
});

// Form submission feedback
document.querySelectorAll('form').forEach(form => {
    form.addEventListener('submit', (e) => {
        const button = form.querySelector('button[type="submit"]');
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = 'Processing...';

        // Re-enable after 3s if form hasn't redirected
        setTimeout(() => {
            button.disabled = false;
            button.textContent = originalText;
        }, 3000);
    });
});

// Link form enhancement
document.querySelectorAll('.link-form').forEach(form => {
    const select = form.querySelector('select[name="target"]');
    const typeInput = form.querySelector('input[name="type"]');
    
    // Auto-focus type input when target is selected
    select.addEventListener('change', () => {
        if (select.value) {
            typeInput.focus();
        }
    });
});

// Card hover effect
document.querySelectorAll('.content-card').forEach(card => {
    card.addEventListener('mouseenter', () => {
        card.style.transform = 'translateY(-2px)';
        card.style.boxShadow = '0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)';
        card.style.transition = 'all 0.2s ease';
    });

    card.addEventListener('mouseleave', () => {
        card.style.transform = 'none';
        card.style.boxShadow = '';
    });
});
