import { colors } from "@staykey/tokens";
import { useEffect } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  type SharedValue,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { font } from "@/components/ui";
import { spring } from "@/lib/motion";
import { STAGES } from "./flow";

/** Stage label and a four-part bar whose Ember fill springs forward or back with each step. */
export function StageHeader({ progress, stageIndex }: { progress: number; stageIndex: number }) {
  const value = useSharedValue(progress);

  useEffect(() => {
    value.value = withSpring(progress, spring.bar);
  }, [progress, value]);

  const stage = STAGES[stageIndex];

  return (
    <View
      style={s.wrap}
      accessibilityRole="progressbar"
      accessibilityLabel={`${stage?.label}, stage ${stageIndex + 1} of ${STAGES.length}`}
    >
      <View style={s.labels}>
        <Animated.Text
          key={stage?.id}
          entering={FadeIn.duration(220)}
          exiting={FadeOut.duration(120)}
          style={s.stage}
        >
          {stage?.label}
        </Animated.Text>
        <Text style={s.count}>
          Stage {stageIndex + 1} of {STAGES.length}
        </Text>
      </View>
      <View style={s.bar}>
        {STAGES.map((st, i) => (
          <Segment key={st.id} index={i} progress={value} />
        ))}
      </View>
    </View>
  );
}

function Segment({ index, progress }: { index: number; progress: SharedValue<number> }) {
  const fill = useAnimatedStyle(() => {
    const amount = Math.min(1, Math.max(0, progress.value - index));
    return { width: `${amount * 100}%` };
  });
  return (
    <View style={s.track}>
      <Animated.View style={[s.fill, fill]} />
    </View>
  );
}

const s = StyleSheet.create({
  wrap: { paddingHorizontal: 20, gap: 8 },
  labels: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    height: 20,
  },
  stage: { fontFamily: font.medium, fontSize: 13, color: colors.obsidian },
  count: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  bar: { flexDirection: "row", gap: 6 },
  track: { flex: 1, height: 4, borderRadius: 2, backgroundColor: colors.cloud, overflow: "hidden" },
  fill: { height: 4, borderRadius: 2, backgroundColor: colors.ember },
});
