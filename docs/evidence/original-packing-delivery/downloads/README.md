# Packing Checklist

A simple, offline-first packing checklist that runs entirely in your browser.
No internet connection or installation required.

## Opening the app

1. Locate the `packing-checklist.html` file in this folder.
2. Double-click it, or drag it into any modern web browser (Chrome, Firefox, Safari, Edge, etc.).
3. The app opens as a local `file://` URL — no server needed.

## Adding items

- Type an item name into the text box (e.g. *Passport*).
- Press **Enter** or click the **Add** button.
- The item appears in the list immediately and is saved automatically.

## Marking items as packed

- Click the **checkbox** next to an item to mark it packed.
- Packed items are shown with a strikethrough to make them easy to spot.
- Click the checkbox again to unmark an item.

## Removing items

- Click the **Remove** button on the right side of any item to delete it from the list.

## Clearing all data

- Click the **🗑 Reset / Clear All** button at the bottom to remove every item at once.
  A confirmation prompt appears before anything is deleted.
- Alternatively, you can clear the saved data directly in your browser:
  open **Developer Tools → Application → Local Storage**, find the entry for this file,
  and delete it (or clear all local storage for the page).

## How data is stored

All items and their packed states are saved in your browser's **localStorage** under the
key `packing-checklist-v1`. Data persists across page refreshes and browser restarts
as long as you use the same browser on the same device.
