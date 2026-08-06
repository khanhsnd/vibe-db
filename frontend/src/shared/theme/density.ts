export const ROW_HEIGHT = 28

/**
 * A grid drawn to be *scanned* rather than worked in. It is what a comparison uses: two panes and a
 * script below them are three things sharing one pane's height, and every pixel a row does not take
 * is another row the reader can see at once. Nothing is dropped to get it — the header keeps its
 * name and its type on one line instead of two, and the cells keep their font.
 */
export const DENSE_ROW_HEIGHT = 22
export const DENSE_HEADER_HEIGHT = 24
export const DENSE_GUTTER_WIDTH = 34

/** A tree's row. Shorter than a grid's: it carries one name, and a sidebar is read as a list. */
export const TREE_ROW_HEIGHT = 24
