// Starry Background Generation
function initStarryBackground() {
    const containers = document.getElementsByClassName('starry-container')
    if (!containers) return;
    console.log(containers)


    for (const container of containers) {

    // Generate a random number between min and max values
    const genRandomNumber = (min, max) => {
        return Math.random() * (max - min) + min;
    };

    // Generate a star <div>
    const genStar = () => {
        const star = document.createElement("div");
        star.classList.add("dynamic-star");

        // Randomly add lantern class to about 50% of stars
        if (Math.random() < 0.50) {
            star.classList.add("lantern");
        }

        // Gen star coordinates relative to container size
        let x = genRandomNumber(1, container.offsetWidth);
        let y = genRandomNumber(1, container.offsetHeight);

        const { style } = star;

        style.left = Math.floor(x) + "px";
        style.top = Math.floor(y) + "px";

        style.setProperty(
            "--star-size",
            genRandomNumber(1, 3) + "px"
        );

        style.setProperty(
            "--twinkle-duration",
            Math.ceil(genRandomNumber(1, 5)) + "s"
        );

        style.setProperty(
            "--twinkle-delay",
            Math.ceil(genRandomNumber(1, 5)) + "s"
        );

        return star;
    };

    // Clear any existing stars
    container.innerHTML = '';

    // Generate 200 stars
    for (let index = 0; index < 200; index++) {
        container.append(genStar());
    }
    
    }
}

// Copy to Clipboard functionality
function copyToClipboard(text, button) {
    navigator.clipboard.writeText(text).then(() => {
        const originalText = button.textContent;
        button.textContent = '✓';
        button.style.color = '#3A7D44';
        
        setTimeout(() => {
            button.textContent = originalText;
            button.style.color = '';
        }, 2000);
    }).catch(err => {
        console.error('Failed to copy text: ', err);
    });
}

// Tabs functionality
function initTabs() {
    const tabTriggers = document.querySelectorAll('.tab-trigger');
    const tabContents = document.querySelectorAll('.tab-content');

    tabTriggers.forEach(trigger => {
        trigger.addEventListener('click', () => {
            const tabId = trigger.getAttribute('data-tab');
            
            // Remove active class from all triggers and contents
            tabTriggers.forEach(t => t.classList.remove('active'));
            tabContents.forEach(c => c.classList.remove('active'));
            
            // Add active class to clicked trigger and corresponding content
            trigger.classList.add('active');
            document.getElementById(`tab-${tabId}`).classList.add('active');
        });
    });
}

// Smooth scrolling for navigation links
function initSmoothScrolling() {
    const navLinks = document.querySelectorAll('a[href^="#"]');
    
    navLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const targetId = link.getAttribute('href').substring(1);
            const targetElement = document.getElementById(targetId);
            
            if (targetElement) {
                targetElement.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
}

// Regenerate stars on window resize
function handleResize() {
    let resizeTimer;
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => {
            initStarryBackground();
        }, 250);
    });
}

// Initialize everything when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    initStarryBackground();
    initTabs();
    initSmoothScrolling();
    handleResize();
});

// Make copyToClipboard globally available for onclick handlers
window.copyToClipboard = copyToClipboard;