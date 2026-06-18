package com.checkmate.security;

import com.fasterxml.jackson.databind.ObjectMapper;
import de.fraunhofer.aisec.cpg.TranslationConfiguration;
import de.fraunhofer.aisec.cpg.TranslationManager;
import de.fraunhofer.aisec.cpg.TranslationResult;
import de.fraunhofer.aisec.cpg.graph.Node;
import de.fraunhofer.aisec.cpg.graph.declarations.FunctionDeclaration;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.MemberCallExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.MemberExpression;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.Literal;
import de.fraunhofer.aisec.cpg.graph.statements.expressions.SubscriptExpression;
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

        // Walk all AST nodes looking for potential issues
        for (var component : result.getComponents()) {
            List<Node> nodes = SubgraphWalker.INSTANCE.flattenAST(component, n -> true);
            for (Node node : nodes) {
                // Detect potential null-pointer dereference patterns:
                // member access on a literal null
                if (node instanceof MemberCallExpression mce) {
                    Node base = mce.getBase();
                    if (base instanceof Literal<?> lit && lit.getValue() == null) {
                        addFinding(findings, "CPG_NPE", node,
                                "Null pointer: method call on null value");
                    }
                } else if (node instanceof MemberExpression me) {
                    Node base = me.getBase();
                    if (base instanceof Literal<?> lit && lit.getValue() == null) {
                        addFinding(findings, "CPG_NPE", node,
                                "Null pointer: member access on null value");
                    }
                }
                // Detect array/subscript access with negative literal index
                if (node instanceof SubscriptExpression sub) {
                    Node idx = sub.getSubscriptExpression();
                    if (idx instanceof Literal<?> lit && lit.getValue() instanceof Number num) {
                        if (num.intValue() < 0) {
                            addFinding(findings, "CPG_OOB", node,
                                    "Out of bounds: negative array index " + num);
                        }
                    }
                }
            }
        }

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("findings", findings);
        payload.put("components", result.getComponents().size());

        new ObjectMapper().writeValue(System.out, payload);
    }

    private static void addFinding(List<Map<String, Object>> findings,
                                    String code, Node node, String message) {
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
