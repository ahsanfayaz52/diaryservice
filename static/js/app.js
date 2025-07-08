document.addEventListener("DOMContentLoaded", function () {
    const cards = document.querySelectorAll(".note-card");

    cards.forEach(card => {
        const content = card.querySelector("p");
        if (content.scrollHeight > content.clientHeight) {
            const readMore = document.createElement("div");
            readMore.textContent = "Read more";
            readMore.className = "read-more";
            card.appendChild(readMore);

            readMore.addEventListener("click", () => {
                card.classList.toggle("expanded");
                readMore.textContent = card.classList.contains("expanded") ? "Show less" : "Read more";
            });
        }
    });
});