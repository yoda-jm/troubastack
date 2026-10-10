import React from "react";
import { Composition, staticFile } from "remotion";
import { Episode, Plan } from "./Episode";

// One composition per episode; its plan (edit.json) is produced by video/build_edit.py into public/epNN/.
export const Root: React.FC = () => (
  <>
    {["ep02", "ep03"].map((id) => (
    <Composition
      key={id}
      id={id}
      component={Episode}
      width={1920}
      height={1080}
      fps={30}
      durationInFrames={30}
      defaultProps={{ planUrl: staticFile(`${id}/edit.json`) }}
      calculateMetadata={async ({ props }) => {
        const plan: Plan = await (await fetch(props.planUrl)).json();
        return { durationInFrames: plan.durationInFrames, props: { ...props, plan } };
      }}
    />
    ))}
  </>
);
