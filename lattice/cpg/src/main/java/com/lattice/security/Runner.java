package com.lattice.security;

import com.fasterxml.jackson.databind.ObjectMapper;
import de.fraunhofer.aisec.cpg.TranslationConfiguration;
import de.fraunhofer.aisec.cpg.TranslationManager;
import de.fraunhofer.aisec.cpg.TranslationResult;
import de.fraunhofer.aisec.cpg.graph.Node;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.CastExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.Literal;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.MemberCallExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.MemberExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.SubscriptExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.UnaryOperator;
import de.fraunhofer.aisec.cpg.helpers.SubgraphWalker;
import java.io.File;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class Runner {

    private Runner() {}

    public static void main(String[] args) throws Exception {
        if (args.length < 1) {
            System.err.println("usage: Runner <code-dir>");
            System.exit(2);
        }

        File codeDir = new File(args[0]);
        if (!codeDir.exists()) {
            System.err.println("code directory does not exist: " + codeDir.getAbsolutePath());
            System.exit(2);
        }

        TranslationConfiguration config =
                TranslationConfiguration.Companion.builder()
                        .sourceLocations(codeDir)
                        .defaultPasses()
                        .registerLanguage("de.fraunhofer.aisec.cpg.frontends.java.JavaLanguage")
                        .build();

        TranslationResult result =
                TranslationManager.Companion.builder().config(config).build().analyze().get();

        List<Map<String, Object>> findings = new ArrayList<>();

        // Walk translation units. Note: flattenAST's predicate is an *exclusion*
        // filter (true => skip), so pass n -> false to visit every node.
        for (var component : result.getComponents()) {
            for (var tu : component.getTranslationUnits()) {
                List<Node> nodes = SubgraphWalker.INSTANCE.flattenAST(tu, n -> false);
                for (Node node : nodes) {
                    analyzeNode(findings, node);
                }
            }
        }

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("findings", findings);
        payload.put("components", result.getComponents().size());
        payload.put("translation_units", result.getTranslationUnits().size());

        new ObjectMapper().writeValue(System.out, payload);
    }

    private static void analyzeNode(List<Map<String, Object>> findings, Node node) {
        // Null-pointer: member access/call whose base unwraps to a null literal
        // (including casted nulls like ((String) null).length()).
        if (node instanceof MemberCallExpression mce) {
            if (isNullLiteral(unwrapCasts(mce.getBase()))) {
                addFinding(findings, "CPG_NPE", node, "Null pointer: method call on null value");
            }
        } else if (node instanceof MemberExpression me) {
            if (isNullLiteral(unwrapCasts(me.getBase()))) {
                addFinding(findings, "CPG_NPE", node, "Null pointer: member access on null value");
            }
        }

        // Out-of-bounds: subscript with a negative constant index (literal or unary minus).
        if (node instanceof SubscriptExpression sub) {
            Integer idx = constantIntValue(sub.getSubscriptExpression());
            if (idx != null && idx < 0) {
                addFinding(
                        findings,
                        "CPG_OOB",
                        node,
                        "Out of bounds: negative array index " + idx);
            }
        }
    }

    /** Strip nested cast expressions so ((T) null) resolves to the null literal. */
    private static Node unwrapCasts(Node node) {
        while (node instanceof CastExpression cast) {
            node = cast.getExpression();
        }
        return node;
    }

    private static boolean isNullLiteral(Node node) {
        return node instanceof Literal<?> lit && lit.getValue() == null;
    }

    /**
     * Resolve a compile-time integer constant from a node, handling plain
     * numeric literals and unary minus (e.g. {@code -1} as UnaryOperator("-", Literal(1))).
     */
    private static Integer constantIntValue(Node node) {
        if (node == null) {
            return null;
        }
        if (node instanceof Literal<?> lit && lit.getValue() instanceof Number num) {
            return num.intValue();
        }
        if (node instanceof UnaryOperator unary) {
            String op = unary.getOperatorCode();
            if ("-".equals(op) || "−".equals(op)) {
                Integer inner = constantIntValue(unary.getInput());
                if (inner != null) {
                    return -inner;
                }
            }
            if ("+".equals(op)) {
                return constantIntValue(unary.getInput());
            }
        }
        return null;
    }

    private static void addFinding(
            List<Map<String, Object>> findings, String code, Node node, String message) {
        Map<String, Object> finding = new LinkedHashMap<>();
        finding.put("code", code);
        String file = "";
        int line = 0;
        if (node.getLocation() != null && node.getLocation().getArtifactLocation() != null) {
            var uri = node.getLocation().getArtifactLocation().getUri();
            if (uri != null) {
                file = uri.getPath() != null ? uri.getPath() : uri.toString();
            }
        }
        if (node.getLocation() != null && node.getLocation().getRegion() != null) {
            line = node.getLocation().getRegion().startLine;
        }
        finding.put("file", file);
        finding.put("line", line);
        finding.put("message", message);
        findings.add(finding);
    }
}
