// CPG targets: null-pointer dereference + out-of-bounds index.
// Pattern matches go-checkmate CPG Runner (MemberCall on null, negative subscript).
public class Vuln {
    public int npeCast() {
        return ((String) null).length();
    }

    public int npeMember() {
        String s = null;
        return s.hashCode();
    }

    public int oob() {
        int[] arr = new int[]{1, 2, 3};
        return arr[-1];
    }

    // Intentionally same-line NPE so file-line dedupe can collapse duplicates if
    // multiple analyzers report the same location (synthetic double for demo via CPG only).
    public int sameLineNpe() { return ((String) null).length(); }
}
