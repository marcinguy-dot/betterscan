package checkmate.cpg;

import com.fasterxml.jackson.databind.ObjectMapper;
import de.fraunhofer.aisec.cpg.TranslationConfiguration;
import de.fraunhofer.aisec.cpg.TranslationManager;
import de.fraunhofer.aisec.cpg.TranslationResult;
import de.fraunhofer.aisec.cpg.analysis.NullPointerCheck;
import de.fraunhofer.aisec.cpg.analysis.OutOfBoundsCheck;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public final class Runner {
    private static final Pattern ANSI = Pattern.compile("\u001B\\[[;\\d]*m");
    private static final Pattern LOCATION =
            Pattern.compile("([\\w./\\\\\\-]+):(\\d+)(?::(\\d+))?");

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
                TranslationConfiguration.builder()
                        .sourceLocations(codeDir)
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.cxx.CLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.cxx.CPPLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.java.JavaLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.llvm.LLVMIRLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.python.PythonLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.golang.GoLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.typescript.TypeScriptLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.ruby.RubyLanguage")
                        .optionalLanguage("de.fraunhofer.aisec.cpg.frontends.ini.IniFileLanguage")
                        .defaultPasses()
                        .useParallelPasses(false)
                        .build();

        TranslationResult result =
                TranslationManager.builder().config(config).build().analyze().get();

        List<Map<String, Object>> findings = new ArrayList<>();
        findings.addAll(captureCheckOutput("CPG_NPE", "Null pointer", new NullPointerCheck(), result));
        findings.addAll(captureCheckOutput("CPG_OOB", "Out of bounds", new OutOfBoundsCheck(), result));

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("findings", findings);
        payload.put("translation_units", result.getComponents().size());

        new ObjectMapper().writeValue(System.out, payload);
    }

    private static List<Map<String, Object>> captureCheckOutput(
            String codePrefix, String label, Object check, TranslationResult result)
            throws Exception {
        ByteArrayOutputStream buffer = new ByteArrayOutputStream();
        PrintStream capture = new PrintStream(buffer, true, StandardCharsets.UTF_8);
        PrintStream original = System.out;
        System.setOut(capture);
        try {
            if (check instanceof NullPointerCheck) {
                ((NullPointerCheck) check).run(result);
            } else if (check instanceof OutOfBoundsCheck) {
                ((OutOfBoundsCheck) check).run(result);
            }
        } finally {
            System.setOut(original);
        }
        return parseFindings(codePrefix, label, buffer.toString(StandardCharsets.UTF_8));
    }

    private static List<Map<String, Object>> parseFindings(
            String codePrefix, String label, String output) {
        String clean = ANSI.matcher(output).replaceAll("");
        List<Map<String, Object>> findings = new ArrayList<>();
        String[] blocks = clean.split("--- FINDING: ");
        int index = 1;
        for (String block : blocks) {
            if (block.trim().isEmpty()) {
                continue;
            }
            String[] lines = block.split("\\R");
            if (lines.length == 0) {
                continue;
            }
            String header = lines[0].replace("---", "").trim();
            String file = "";
            int line = 0;
            String message = label + ": " + header;
            for (int i = 1; i < lines.length; i++) {
                String candidate = lines[i].trim();
                if (candidate.isEmpty() || candidate.startsWith("The following path")) {
                    continue;
                }
                Matcher matcher = LOCATION.matcher(candidate);
                if (matcher.find()) {
                    file = matcher.group(1);
                    line = Integer.parseInt(matcher.group(2));
                    break;
                }
            }
            Map<String, Object> finding = new LinkedHashMap<>();
            finding.put("code", codePrefix);
            finding.put("file", file);
            finding.put("line", line);
            finding.put("message", message);
            findings.add(finding);
            index++;
        }
        return findings;
    }
}
