# PDF Output

- When the user requests a PDF deliverable, try WeasyPrint first. Use HTML and print CSS as the source.
- If WeasyPrint is missing, install it for the user within the available permissions. Do not stop at installation advice.
- Follow the [official installation instructions](https://doc.courtbouillon.org/weasyprint/stable/first_steps.html#installation) for the current platform and required libraries.
- On macOS with Homebrew, use `brew install weasyprint`. Otherwise, use a supported package manager or an isolated Python environment.
- Check the installation with `weasyprint --info`. Generate the PDF, render its pages, and inspect the layout before delivery.
- Use another tool if the user requires it, or if a required feature or installation failure prevents WeasyPrint use. Explain the reason.
- This preference applies to PDF creation. Use suitable tools for existing PDF operations, such as merging, extraction, or OCR.
