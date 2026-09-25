# Chinese chess

The independent Xiangqi engine uses the standard 9×10 starting layout and red moves first. It implements general, advisor, elephant, horse, chariot, cannon, and soldier movement, including palace and river limits, horse-leg and elephant-eye blocking, cannon screens, and the flying-general rule. A move is rejected if it leaves the moving side's general in check. The engine detects checkmate and stalemate; the terminal client uses the arrow keys to select a square and Enter to select or move a piece.
