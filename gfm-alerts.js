(() => {
  const alerts = {
    NOTE: {
      type: "note",
      title: "Note"
    },
    TIP: {
      type: "tip",
      title: "Tip"
    },
    IMPORTANT: {
      type: "example",
      title: "Important"
    },
    WARNING: {
      type: "warning",
      title: "Warning"
    },
    CAUTION: {
      type: "danger",
      title: "Caution"
    },
  };
  const markerPattern = /^\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*/i;

  function convertAlert(quote) {
    const paragraph = quote.querySelector(":scope > p");
    const match = paragraph?.textContent.match(markerPattern);
    if (!match) {
      return;
    }

    const alert = alerts[match[1].toUpperCase()];
    const textWalker = document.createTreeWalker(paragraph, NodeFilter.SHOW_TEXT);
    const markerNode = textWalker.nextNode();
    if (!markerNode) {
      return;
    }

    // Markdown renders the marker and its body in one paragraph with a <br>.
    markerNode.nodeValue = markerNode.nodeValue.replace(markerPattern, "");
    while (paragraph.firstChild) {
      const first = paragraph.firstChild;
      if (first.nodeType === Node.TEXT_NODE && !first.nodeValue.trim()) {
        first.remove();
        continue;
      }
      if (first.nodeName === "BR") {
        first.remove();
        continue;
      }
      break;
    }

    if (!paragraph.textContent.trim() && !paragraph.querySelector("img, svg, video, audio, iframe")) {
      paragraph.remove();
    }

    const admonition = document.createElement("div");
    admonition.className = `admonition ${alert.type}`;

    const title = document.createElement("p");
    title.className = "admonition-title";
    title.textContent = alert.title;
    admonition.append(title);

    // Move the original nodes so links, code, and nested Markdown stay intact.
    while (quote.firstChild) {
      admonition.append(quote.firstChild);
    }
    quote.replaceWith(admonition);
  }

  function renderAlerts() {
    document.querySelectorAll(".md-typeset blockquote").forEach(convertAlert);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", renderAlerts, {
      once: true
    });
  } else {
    renderAlerts();
  }
})();
