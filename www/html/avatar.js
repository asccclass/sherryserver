import * as THREE from "https://esm.sh/three@0.168.0";
import { GLTFLoader } from "https://esm.sh/three@0.168.0/examples/jsm/loaders/GLTFLoader.js";
import { VRMLoaderPlugin, VRMUtils } from "https://esm.sh/@pixiv/three-vrm@3.3.4?deps=three@0.168.0";

const canvas = document.getElementById("avatarCanvas");
const diag = document.getElementById("avatarDiag");
const mouthMeter = document.getElementById("mouthMeter");

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(24, 1, 0.1, 100);
camera.position.set(0, 1.42, 1.78);

const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.outputColorSpace = THREE.SRGBColorSpace;
camera.lookAt(0, 1.15, 0);

scene.add(new THREE.HemisphereLight(0xffffff, 0xb9a48a, 1.8));
const key = new THREE.DirectionalLight(0xffffff, 1.8);
key.position.set(2.4, 3.6, 2.2);
scene.add(key);

const rim = new THREE.DirectionalLight(0xfff2db, 0.8);
rim.position.set(-1.8, 2.6, -2);
scene.add(rim);

const floor = new THREE.Mesh(
  new THREE.CircleGeometry(1.65, 48),
  new THREE.MeshBasicMaterial({ color: 0xe2d7c4, transparent: true, opacity: 0.28 }),
);
floor.rotation.x = -Math.PI / 2;
floor.position.y = -0.02;
scene.add(floor);

const clock = new THREE.Clock();

const lipState = {
  vrm: null,
  root: null,
  mode: "none",
  targetValue: 0,
  currentValue: 0,
  jawBone: null,
  jawBaseRotationX: 0,
  expressionNames: [],
  vowelWeights: { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 },
  targetVowelWeights: { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 },
  visemeWeights: {
    viseme_aa: 0,
    viseme_E: 0,
    viseme_I: 0,
    viseme_O: 0,
    viseme_U: 0,
    viseme_PP: 0,
    viseme_SS: 0,
    viseme_TH: 0,
    viseme_DD: 0,
    viseme_FF: 0,
    viseme_kk: 0,
    viseme_nn: 0,
    viseme_RR: 0,
    viseme_CH: 0,
    viseme_sil: 0,
  },
  visemeModeActive: false,
  headBone: null,
  neckBone: null,
  spineBone: null,
  chestBone: null,
  hipsBone: null,
  leftUpperArmBone: null,
  rightUpperArmBone: null,
  leftLowerArmBone: null,
  rightLowerArmBone: null,
  headBaseEuler: null,
  neckBaseEuler: null,
  spineBaseEuler: null,
  chestBaseEuler: null,
  hipsBaseEuler: null,
  leftUpperArmBaseEuler: null,
  rightUpperArmBaseEuler: null,
  leftLowerArmBaseEuler: null,
  rightLowerArmBaseEuler: null,
  bodyBaseY: 0,
  bodyBaseX: 0,
  walkCycle: 0,
};

let currentAvatarModel = "";
let currentAvatarRoot = null;

const VOWEL_EXPRESSION_PATTERNS = {
  aa: [/^aa$/i, /^a$/i, /mouth.*aa/i, /viseme[_-]?aa/i, /a[_-]?mouth/i],
  ih: [/^ih$/i, /^i$/i, /mouth.*ih/i, /viseme[_-]?ih/i, /i[_-]?mouth/i],
  ou: [/^ou$/i, /^u$/i, /mouth.*ou/i, /viseme[_-]?ou/i, /u[_-]?mouth/i],
  ee: [/^ee$/i, /^e$/i, /mouth.*ee/i, /viseme[_-]?ee/i, /e[_-]?mouth/i],
  oh: [/^oh$/i, /^o$/i, /mouth.*oh/i, /viseme[_-]?oh/i, /o[_-]?mouth/i],
};

const VOWEL_RESPONSE = {
  aa: { gain: 1.34, ease: 24 },
  ih: { gain: 1.02, ease: 20 },
  ou: { gain: 1.12, ease: 19 },
  ee: { gain: 0.96, ease: 18 },
  oh: { gain: 1.18, ease: 20 },
};

const introState = {
  active: true,
  elapsed: 0,
  duration: 5.2,
  startPosition: new THREE.Vector3(0, 0, -4.2),
  endPosition: new THREE.Vector3(0, 0, 0.2),
  startYaw: Math.PI * 0.92,
  endYaw: Math.PI,
};

const cameraIntro = {
  startPosition: new THREE.Vector3(0.08, 1.5, 2.48),
  endPosition: new THREE.Vector3(0.01, 1.72, 1.3),
  startLookAt: new THREE.Vector3(0, 1.08, 0),
  endLookAt: new THREE.Vector3(0, 1.68, 0),
};

const PRESENTATION_EXPRESSION_PATTERNS = {
  smile: [/smile/i, /happy/i, /joy/i, /grin/i],
  blink: [/^blink$/i, /blink/i, /eye.*close/i],
  blinkLeft: [/blink.*left/i, /^blink_l$/i, /eye.*left.*close/i],
  blinkRight: [/blink.*right/i, /^blink_r$/i, /eye.*right.*close/i],
};

function resize() {
  const rect = canvas.getBoundingClientRect();
  const width = Math.max(1, Math.floor(rect.width));
  const height = Math.max(1, Math.floor(rect.height));
  renderer.setSize(width, height, false);
  camera.aspect = width / height;
  camera.updateProjectionMatrix();
}

function setDiag(text) {
  diag.textContent = text;
}

function findJawBone(root) {
  let jawBone = null;
  root.traverse((obj) => {
    if (!jawBone && obj.isBone && /jaw|mouth/i.test(obj.name)) {
      jawBone = obj;
    }
  });
  return jawBone;
}

function findFirstBone(root, patterns) {
  let bone = null;
  root.traverse((obj) => {
    if (bone || !obj.isBone) return;
    if (patterns.some((pattern) => pattern.test(obj.name))) {
      bone = obj;
    }
  });
  return bone;
}

function cloneEuler(euler) {
  return new THREE.Euler(euler.x, euler.y, euler.z, euler.order);
}

function smoothStep01(value) {
  const t = THREE.MathUtils.clamp(value, 0, 1);
  return t * t * (3 - 2 * t);
}

function easeOutCubic(value) {
  const t = THREE.MathUtils.clamp(value, 0, 1);
  return 1 - Math.pow(1 - t, 3);
}

function inspectVRM(vrm) {
  const root = vrm.scene;
  const expressionManager = vrm.expressionManager;
  const jawBone = findJawBone(root);
  const headBone = findFirstBone(root, [/head/i]);
  const neckBone = findFirstBone(root, [/neck/i]);
  const spineBone = findFirstBone(root, [/spine/i]);
  const chestBone = findFirstBone(root, [/chest|upperchest/i]);
  const hipsBone = findFirstBone(root, [/hips|pelvis/i]);
  const leftUpperArmBone = findFirstBone(root, [/left.*upperarm/i, /upperarm.*left/i, /l.*upperarm/i, /left.*arm/i]);
  const rightUpperArmBone = findFirstBone(root, [/right.*upperarm/i, /upperarm.*right/i, /r.*upperarm/i, /right.*arm/i]);
  const leftLowerArmBone = findFirstBone(root, [/left.*lowerarm/i, /lowerarm.*left/i, /left.*forearm/i, /l.*forearm/i]);
  const rightLowerArmBone = findFirstBone(root, [/right.*lowerarm/i, /lowerarm.*right/i, /right.*forearm/i, /r.*forearm/i]);
  const expressionNames = expressionManager
    ? Array.from(expressionManager.expressionMap.keys ? expressionManager.expressionMap.keys() : Object.keys(expressionManager.expressionMap || {}))
    : [];

  lipState.vrm = vrm;
  lipState.root = root;
  lipState.jawBone = jawBone;
  lipState.jawBaseRotationX = jawBone ? jawBone.rotation.x : 0;
  lipState.expressionNames = expressionNames;
  lipState.headBone = headBone;
  lipState.neckBone = neckBone;
  lipState.spineBone = spineBone;
  lipState.chestBone = chestBone;
  lipState.hipsBone = hipsBone;
  lipState.leftUpperArmBone = leftUpperArmBone;
  lipState.rightUpperArmBone = rightUpperArmBone;
  lipState.leftLowerArmBone = leftLowerArmBone;
  lipState.rightLowerArmBone = rightLowerArmBone;
  lipState.headBaseEuler = headBone ? cloneEuler(headBone.rotation) : null;
  lipState.neckBaseEuler = neckBone ? cloneEuler(neckBone.rotation) : null;
  lipState.spineBaseEuler = spineBone ? cloneEuler(spineBone.rotation) : null;
  lipState.chestBaseEuler = chestBone ? cloneEuler(chestBone.rotation) : null;
  lipState.hipsBaseEuler = hipsBone ? cloneEuler(hipsBone.rotation) : null;
  lipState.leftUpperArmBaseEuler = leftUpperArmBone ? cloneEuler(leftUpperArmBone.rotation) : null;
  lipState.rightUpperArmBaseEuler = rightUpperArmBone ? cloneEuler(rightUpperArmBone.rotation) : null;
  lipState.leftLowerArmBaseEuler = leftLowerArmBone ? cloneEuler(leftLowerArmBone.rotation) : null;
  lipState.rightLowerArmBaseEuler = rightLowerArmBone ? cloneEuler(rightLowerArmBone.rotation) : null;
  lipState.bodyBaseY = root.position.y;
  lipState.bodyBaseX = root.position.x;

  const hasAa = expressionNames.some((name) => /aa|a|oh|ou|ee|ih|mouthopen|jawopen/i.test(name));
  lipState.mode = hasAa ? "vrm-expression" : jawBone ? "jaw" : "none";

  const lines = [];
  lines.push(`Avatar file: ${currentAvatarModel.split("/").pop() || "unknown"}`);
  lines.push(`Lip-sync mode: ${lipState.mode}`);
  lines.push(`VRM expressions: ${expressionNames.length}`);
  if (expressionNames.length) {
    lines.push(`Expression names: ${expressionNames.join(", ")}`);
  }
  lines.push(`Jaw bone: ${jawBone ? jawBone.name : "not found"}`);
  lines.push(`Head bone: ${headBone ? headBone.name : "not found"}`);
  lines.push(`Upper arm bones: ${leftUpperArmBone ? leftUpperArmBone.name : "L not found"} / ${rightUpperArmBone ? rightUpperArmBone.name : "R not found"}`);
  if (lipState.mode === "none") {
    lines.push("");
    lines.push("VRM loaded, but no obvious mouth expression or jaw bone was detected.");
  }
  setDiag(lines.join("\n"));
}

function setExpressionValue(name, value) {
  if (!lipState.vrm?.expressionManager) return false;
  try {
    lipState.vrm.expressionManager.setValue(name, value);
    return true;
  } catch {
    return false;
  }
}

function findExpressionName(patterns) {
  return lipState.expressionNames.find((name) => patterns.some((pattern) => pattern.test(name))) || null;
}

function setMappedExpression(patterns, value) {
  const name = findExpressionName(patterns);
  if (!name) return false;
  return setExpressionValue(name, value);
}

function setOptionalExpression(patterns, value) {
  setMappedExpression(patterns, value);
}

function resetTrackedExpressions() {
  for (const patterns of Object.values(VOWEL_EXPRESSION_PATTERNS)) {
    setMappedExpression(patterns, 0);
  }
}

function shapeVowelWeights(rawPose) {
  const pose = {
    aa: THREE.MathUtils.clamp(rawPose.aa || 0, 0, 1),
    ih: THREE.MathUtils.clamp(rawPose.ih || 0, 0, 1),
    ou: THREE.MathUtils.clamp(rawPose.ou || 0, 0, 1),
    ee: THREE.MathUtils.clamp(rawPose.ee || 0, 0, 1),
    oh: THREE.MathUtils.clamp(rawPose.oh || 0, 0, 1),
  };

  const dominant = Math.max(pose.aa, pose.ih, pose.ou, pose.ee, pose.oh, 0);
  if (dominant <= 0) {
    return pose;
  }

  const sharpened = {};
  for (const key of Object.keys(pose)) {
    const normalized = pose[key] / dominant;
    const emphasized = Math.pow(normalized, key === "aa" ? 1.18 : 1.32);
    const weighted = emphasized * VOWEL_RESPONSE[key].gain * dominant;
    sharpened[key] = THREE.MathUtils.clamp(weighted, 0, 1);
  }

    const primary = Object.entries(sharpened).sort((a, b) => b[1] - a[1])[0]?.[0];
    if (primary) {
      for (const key of Object.keys(sharpened)) {
        if (key !== primary) {
          sharpened[key] *= 0.48;
        }
      }
    }

    if (sharpened.aa > 0) {
    sharpened.oh *= 0.66;
    sharpened.ou *= 0.58;
    }
    if (sharpened.ee > 0 || sharpened.ih > 0) {
    sharpened.aa *= 0.72;
    sharpened.oh *= 0.6;
    sharpened.ou *= 0.56;
    }
    if (sharpened.ou > 0 || sharpened.oh > 0) {
    sharpened.ee *= 0.46;
    sharpened.ih *= 0.58;
    }

  return {
    aa: THREE.MathUtils.clamp(sharpened.aa, 0, 1),
    ih: THREE.MathUtils.clamp(sharpened.ih, 0, 1),
    ou: THREE.MathUtils.clamp(sharpened.ou, 0, 1),
    ee: THREE.MathUtils.clamp(sharpened.ee, 0, 1),
    oh: THREE.MathUtils.clamp(sharpened.oh, 0, 1),
  };
}

function getPoseFromVisemes() {
  const v = lipState.visemeWeights;
  const closure = THREE.MathUtils.clamp(
    v.viseme_PP * 1.1 +
    v.viseme_sil * 0.95 +
    v.viseme_FF * 0.18 +
    v.viseme_TH * 0.08,
    0,
    1,
  );

  const pose = {
    aa: v.viseme_aa * 1.28 + v.viseme_DD * 0.26 + v.viseme_kk * 0.18 + v.viseme_CH * 0.16 + v.viseme_RR * 0.1,
    ih: v.viseme_I * 1.08 + v.viseme_E * 0.42 + v.viseme_SS * 0.38 + v.viseme_TH * 0.22 + v.viseme_nn * 0.18,
    ou: v.viseme_U * 1.18 + v.viseme_O * 0.46 + v.viseme_RR * 0.24,
    ee: v.viseme_E * 1.02 + v.viseme_I * 0.84 + v.viseme_SS * 0.42 + v.viseme_FF * 0.22 + v.viseme_CH * 0.22,
    oh: v.viseme_O * 1.12 + v.viseme_U * 0.36 + v.viseme_aa * 0.24 + v.viseme_RR * 0.18,
  };

  const openness = Math.max(0.08, 1 - closure * 0.92);
  return shapeVowelWeights({
    aa: pose.aa * openness * 1.16,
    ih: pose.ih * openness * 1.08,
    ou: pose.ou * openness * 1.1,
    ee: pose.ee * openness * 1.05,
    oh: pose.oh * openness * 1.1,
  });
}

function applyMouthPose(pose) {
  lipState.visemeModeActive = false;
  lipState.targetVowelWeights = shapeVowelWeights(pose);
  const maxValue = Math.max(...Object.values(lipState.targetVowelWeights), 0);
  lipState.targetValue = maxValue;
  mouthMeter.style.width = `${Math.round(maxValue * 100)}%`;
}

function applyMouthValue(value) {
  const clamped = THREE.MathUtils.clamp(value, 0, 1);
  applyMouthPose({
    aa: clamped * 0.94,
    ih: clamped * 0.32,
    ou: clamped * 0.38,
    ee: clamped * 0.22,
    oh: clamped * 0.48,
  });
}

function setVisemeWeight(key, value) {
  if (!(key in lipState.visemeWeights)) {
    return;
  }
  lipState.visemeModeActive = true;
  lipState.visemeWeights[key] = THREE.MathUtils.clamp(value || 0, 0, 1);
  lipState.targetVowelWeights = getPoseFromVisemes();
  lipState.targetValue = Math.max(...Object.values(lipState.targetVowelWeights), 0);
  mouthMeter.style.width = `${Math.round(lipState.targetValue * 100)}%`;
}

function clearVisemes() {
  lipState.visemeModeActive = false;
  for (const key of Object.keys(lipState.visemeWeights)) {
    lipState.visemeWeights[key] = 0;
  }
  lipState.targetVowelWeights = { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 };
  lipState.targetValue = 0;
  mouthMeter.style.width = "0%";
}

function updateLipSync(dt) {
  lipState.currentValue = THREE.MathUtils.lerp(
    lipState.currentValue,
    lipState.targetValue,
    1 - Math.exp(-dt * 16),
  );
  for (const key of Object.keys(lipState.vowelWeights)) {
    lipState.vowelWeights[key] = THREE.MathUtils.lerp(
      lipState.vowelWeights[key],
      lipState.targetVowelWeights[key],
      1 - Math.exp(-dt * VOWEL_RESPONSE[key].ease),
    );
  }

  if (lipState.mode === "vrm-expression" && lipState.vrm?.expressionManager) {
    resetTrackedExpressions();
    setMappedExpression(VOWEL_EXPRESSION_PATTERNS.aa, lipState.vowelWeights.aa);
    setMappedExpression(VOWEL_EXPRESSION_PATTERNS.ih, lipState.vowelWeights.ih);
    setMappedExpression(VOWEL_EXPRESSION_PATTERNS.ou, lipState.vowelWeights.ou);
    setMappedExpression(VOWEL_EXPRESSION_PATTERNS.ee, lipState.vowelWeights.ee);
    setMappedExpression(VOWEL_EXPRESSION_PATTERNS.oh, lipState.vowelWeights.oh);
  } else if (lipState.mode === "jaw" && lipState.jawBone) {
    lipState.jawBone.rotation.x = lipState.jawBaseRotationX + lipState.currentValue * 0.35;
  }

  const t = performance.now() * 0.001;
  let introBlend = 1;
  let walkWeight = 0;
  let introEased = 1;

  if (introState.active && lipState.root) {
    introState.elapsed += dt;
    const progress = THREE.MathUtils.clamp(introState.elapsed / introState.duration, 0, 1);
    const eased = easeOutCubic(progress);
    introEased = eased;
    introBlend = eased;
    walkWeight = 1 - eased;

    lipState.root.position.x = THREE.MathUtils.lerp(introState.startPosition.x, introState.endPosition.x, eased);
    lipState.root.position.z = THREE.MathUtils.lerp(introState.startPosition.z, introState.endPosition.z, eased);
    lipState.root.rotation.y = THREE.MathUtils.lerp(introState.startYaw, introState.endYaw, eased);

    lipState.walkCycle += dt * (3.3 - eased * 1.1);

    if (progress >= 1) {
      introState.active = false;
      lipState.root.position.x = introState.endPosition.x;
      lipState.root.position.z = introState.endPosition.z;
      lipState.root.rotation.y = introState.endYaw;
    }
  }

  camera.position.lerpVectors(cameraIntro.startPosition, cameraIntro.endPosition, introEased);
  const cameraTarget = new THREE.Vector3().lerpVectors(cameraIntro.startLookAt, cameraIntro.endLookAt, introEased);
  camera.lookAt(cameraTarget);

  const breathe = Math.sin(t * 1.8) * 0.006;
  const headYaw = Math.sin(t * 0.9) * 0.045 * introBlend;
  const headPitch = Math.sin(t * 1.3 + 0.6) * 0.02 * introBlend;
  const neckYaw = Math.sin(t * 0.7 + 0.8) * 0.018 * introBlend;
  const spinePitch = Math.sin(t * 1.8) * 0.01 * introBlend;
  const walkBob = Math.sin(lipState.walkCycle * 2.1) * 0.03 * walkWeight;
  const walkTorsoPitch = Math.sin(lipState.walkCycle * 2.1) * 0.035 * walkWeight;
  const walkShoulder = Math.sin(lipState.walkCycle * 2.1 + Math.PI * 0.5) * 0.02 * walkWeight;
  const weightShift = Math.sin(lipState.walkCycle * 2.1 + Math.PI * 0.2) * 0.032 * walkWeight;
  const hipYaw = Math.sin(lipState.walkCycle * 2.1 + Math.PI * 0.2) * 0.04 * walkWeight;
  const leftArmSwing = Math.sin(lipState.walkCycle * 2.1 + Math.PI) * 0.35 * walkWeight;
  const rightArmSwing = Math.sin(lipState.walkCycle * 2.1) * 0.35 * walkWeight;
  const leftForearmSwing = Math.max(0, Math.sin(lipState.walkCycle * 2.1 + Math.PI)) * 0.15 * walkWeight;
  const rightForearmSwing = Math.max(0, Math.sin(lipState.walkCycle * 2.1)) * 0.15 * walkWeight;
  const poseBlend = introEased;
  const finalHeadTurn = 0.045 * poseBlend;
  const finalHeadPitch = -0.03 * poseBlend;
  const finalChestLift = -0.025 * poseBlend;
  const finalShoulderSettle = 0.06 * poseBlend;
  const settleStart = 0.84;
  const settleProgress = THREE.MathUtils.clamp((introEased - settleStart) / (1 - settleStart), 0, 1);
  const presentationBlend = smoothStep01(settleProgress);
  const blinkPulse = Math.exp(-Math.pow((presentationBlend - 0.52) / 0.12, 2)) * 0.95;
  const smileAmount = 0.18 * presentationBlend;
  const gazeLockYaw = -0.028 * presentationBlend;
  const gazeLockPitch = -0.018 * presentationBlend;

  if (lipState.root) {
    lipState.root.position.y = lipState.bodyBaseY + breathe + walkBob;
    lipState.root.position.x += weightShift;
  }
  if (lipState.headBone && lipState.headBaseEuler) {
    lipState.headBone.rotation.y = lipState.headBaseEuler.y + headYaw + finalHeadTurn + gazeLockYaw;
    lipState.headBone.rotation.x = lipState.headBaseEuler.x + headPitch + walkTorsoPitch * 0.35 + finalHeadPitch + gazeLockPitch;
    lipState.headBone.rotation.z = lipState.headBaseEuler.z + Math.sin(t * 0.8) * 0.01 * introBlend + walkShoulder * 0.2;
  }
  if (lipState.neckBone && lipState.neckBaseEuler) {
    lipState.neckBone.rotation.y = lipState.neckBaseEuler.y + neckYaw + finalHeadTurn * 0.35 + gazeLockYaw * 0.45;
    lipState.neckBone.rotation.x = lipState.neckBaseEuler.x + headPitch * 0.4 + walkTorsoPitch * 0.3 + finalHeadPitch * 0.3 + gazeLockPitch * 0.35;
  }
  if (lipState.spineBone && lipState.spineBaseEuler) {
    lipState.spineBone.rotation.x = lipState.spineBaseEuler.x + spinePitch + walkTorsoPitch;
    lipState.spineBone.rotation.z = lipState.spineBaseEuler.z + walkShoulder * 0.3;
  }
  if (lipState.chestBone && lipState.chestBaseEuler) {
    lipState.chestBone.rotation.x = lipState.chestBaseEuler.x + breathe * 0.6 + walkTorsoPitch * 0.5 + finalChestLift;
    lipState.chestBone.rotation.z = lipState.chestBaseEuler.z - walkShoulder * 0.2;
  }
  if (lipState.hipsBone && lipState.hipsBaseEuler) {
    lipState.hipsBone.rotation.y = lipState.hipsBaseEuler.y + hipYaw;
    lipState.hipsBone.rotation.z = lipState.hipsBaseEuler.z - weightShift * 1.6;
    lipState.hipsBone.rotation.x = lipState.hipsBaseEuler.x - walkTorsoPitch * 0.15;
  }
  if (lipState.leftUpperArmBone && lipState.leftUpperArmBaseEuler) {
    lipState.leftUpperArmBone.rotation.x = lipState.leftUpperArmBaseEuler.x + leftArmSwing;
    lipState.leftUpperArmBone.rotation.y = lipState.leftUpperArmBaseEuler.y - 0.025 * walkWeight;
    lipState.leftUpperArmBone.rotation.z = lipState.leftUpperArmBaseEuler.z + 1.25 + 0.01 * walkWeight + finalShoulderSettle * 0.12;
  }
  if (lipState.rightUpperArmBone && lipState.rightUpperArmBaseEuler) {
    lipState.rightUpperArmBone.rotation.x = lipState.rightUpperArmBaseEuler.x + rightArmSwing;
    lipState.rightUpperArmBone.rotation.y = lipState.rightUpperArmBaseEuler.y + 0.025 * walkWeight;
    lipState.rightUpperArmBone.rotation.z = lipState.rightUpperArmBaseEuler.z - 1.25 - 0.01 * walkWeight - finalShoulderSettle * 0.12;
  }
  if (lipState.leftLowerArmBone && lipState.leftLowerArmBaseEuler) {
    lipState.leftLowerArmBone.rotation.x = lipState.leftLowerArmBaseEuler.x + 0.04 - leftForearmSwing * 0.14;
  }
  if (lipState.rightLowerArmBone && lipState.rightLowerArmBaseEuler) {
    lipState.rightLowerArmBone.rotation.x = lipState.rightLowerArmBaseEuler.x + 0.04 - rightForearmSwing * 0.14;
  }

  if (lipState.mode === "vrm-expression" && lipState.vrm?.expressionManager) {
    setOptionalExpression(PRESENTATION_EXPRESSION_PATTERNS.smile, smileAmount);
    const hasLeft = setMappedExpression(PRESENTATION_EXPRESSION_PATTERNS.blinkLeft, blinkPulse);
    const hasRight = setMappedExpression(PRESENTATION_EXPRESSION_PATTERNS.blinkRight, blinkPulse);
    if (!hasLeft && !hasRight) {
      setOptionalExpression(PRESENTATION_EXPRESSION_PATTERNS.blink, blinkPulse);
    }
  }

  if (lipState.vrm) {
    lipState.vrm.update(dt);
  }
}

function animate() {
  const dt = clock.getDelta();
  updateLipSync(dt);
  renderer.render(scene, camera);
  requestAnimationFrame(animate);
}

async function initAvatar() {
  resize();
  window.addEventListener("resize", resize);
  await loadAvatar("/avatar/friday.vrm");
}

async function loadAvatar(modelPath) {
  const normalizedPath = modelPath || "/avatar/friday.vrm";
  currentAvatarModel = normalizedPath;

  const loader = new GLTFLoader();
  loader.register((parser) => new VRMLoaderPlugin(parser));

  try {
    const gltf = await loader.loadAsync(normalizedPath);
    const vrm = gltf.userData.vrm;
    if (!vrm) {
      throw new Error(`VRM metadata was not found in ${normalizedPath}`);
    }

    const root = vrm.scene;
    VRMUtils.combineSkeletons(root);
    root.rotation.y = introState.startYaw;
    root.position.copy(introState.startPosition);

    const box = new THREE.Box3().setFromObject(root);
    const size = new THREE.Vector3();
    const center = new THREE.Vector3();
    box.getSize(size);
    box.getCenter(center);

    root.position.x -= center.x;
    root.position.z -= center.z;
    root.position.y -= box.min.y;
    root.scale.setScalar(1.92 / Math.max(size.y, 1));

    if (currentAvatarRoot) {
      scene.remove(currentAvatarRoot);
    }
    currentAvatarRoot = root;
    scene.add(root);
    inspectVRM(vrm);
    camera.position.copy(cameraIntro.startPosition);
    camera.lookAt(cameraIntro.startLookAt);
  } catch (error) {
    console.error(error);
    setDiag(`Failed to load ${normalizedPath}\n${error.message}`);
  }
}

window.avatarLipSync = {
  setMouthOpen: applyMouthValue,
  setMouthPose: applyMouthPose,
  setVisemeWeight,
  clearVisemes,
};

window.avatarRuntime = {
  loadAvatar,
};

initAvatar();
animate();
